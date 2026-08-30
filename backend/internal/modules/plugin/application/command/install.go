package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	plugininstall "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gopkg.in/yaml.v3"
)

const (
	backendModulePath = "github.com/sparkops/sparkops-agent/backend"
	registerGoPath    = "backend/internal/plugins/register.go"
)

type InstallHandler struct {
	repo      repository.PluginRepository
	publisher plugininstall.GitPublisher
	root      string
}

func NewInstallHandler(repo repository.PluginRepository, publisher plugininstall.GitPublisher, root ...string) *InstallHandler {
	return &InstallHandler{repo: repo, publisher: publisher, root: firstRoot(root)}
}

func (h *InstallHandler) Handle(ctx context.Context, rawZip []byte, actor kernel.MemberID) (*model.PluginRecord, error) {
	inspected, err := plugininstall.InspectZip(rawZip)
	if err != nil {
		return nil, err
	}

	manifestBytes, err := yaml.Marshal(inspected.Manifest)
	if err != nil {
		return nil, err
	}
	files, err := installFiles(inspected, manifestBytes)
	if err != nil {
		return nil, err
	}
	installedFiles := make([]string, 0, len(files))
	for name := range files {
		installedFiles = append(installedFiles, name)
	}
	sort.Strings(installedFiles)
	digestSum := sha256.Sum256(rawZip)
	packageDigest := hex.EncodeToString(digestSum[:])

	record, findErr := h.repo.FindPlugin(ctx, inspected.Manifest.ID)
	isRetry := findErr == nil
	if findErr != nil && !errors.Is(findErr, domain.ErrPluginNotFound) {
		return nil, findErr
	}
	if isRetry {
		if err := assertRetryIdentity(record, inspected.Manifest, packageDigest, installedFiles); err != nil {
			return nil, err
		}
	} else {
		record = &model.PluginRecord{
			ID:             inspected.Manifest.ID,
			Version:        inspected.Manifest.Version,
			DisplayName:    inspected.Manifest.DisplayName,
			ManifestYAML:   string(manifestBytes),
			PackageDigest:  packageDigest,
			InstalledFiles: installedFiles,
			Status:         model.PluginStatusUploaded,
			CreatedBy:      actor,
		}
		if err := h.repo.SavePlugin(ctx, record); err != nil {
			if !errors.Is(err, domain.ErrPluginIDConflict) {
				return nil, err
			}
			existing, findErr := h.repo.FindPlugin(ctx, inspected.Manifest.ID)
			if findErr != nil {
				return nil, err
			}
			record = existing
			isRetry = true
			if identErr := assertRetryIdentity(record, inspected.Manifest, packageDigest, installedFiles); identErr != nil {
				return nil, identErr
			}
		}
	}
	if isRetry {
		branch := "plugin/install-" + record.ID
		view, err := h.publisher.LookupPullRequest(ctx, branch)
		if err == nil {
			if err := record.MarkPROpen(view.URL); err != nil {
				return nil, err
			}
			if err := h.repo.SavePlugin(ctx, record); err != nil {
				return record, err
			}
			return record, nil
		}
		if !errors.Is(err, plugininstall.ErrPullRequestNotFound) {
			return nil, err
		}
	}

	root, err := resolveInstallRoot(h.root)
	if err != nil {
		return nil, err
	}
	registerPath := filepath.Join(root, filepath.FromSlash(registerGoPath))
	currentRegister, err := os.ReadFile(registerPath)
	if err != nil {
		return nil, fmt.Errorf("read plugin register: %w", err)
	}
	pluginIDs, err := registeredPluginIDs(currentRegister)
	if err != nil {
		return nil, fmt.Errorf("parse plugin register: %w", err)
	}
	pluginIDs = append(pluginIDs, inspected.Manifest.ID)
	nextRegister, err := plugininstall.RewriteRegisterGo(currentRegister, backendModulePath, pluginIDs)
	if err != nil {
		return nil, err
	}
	files[registerGoPath] = nextRegister
	prURL, publishErr := h.publisher.Publish(ctx, plugininstall.PublishRequest{
		Branch:  "plugin/install-" + record.ID,
		Message: "Install plugin " + record.ID,
		Files:   files,
	})
	if publishErr != nil {
		if record.Status != model.PluginStatusPRFailed {
			if err := record.MarkPRFailed(); err != nil {
				return record, errors.Join(publishErr, err)
			}
		}
		if err := h.repo.SavePlugin(ctx, record); err != nil {
			return record, errors.Join(publishErr, err)
		}
		return record, publishErr
	}
	if err := record.MarkPROpen(prURL); err != nil {
		return record, err
	}
	if err := h.repo.SavePlugin(ctx, record); err != nil {
		return record, err
	}
	return record, nil
}

func assertRetryIdentity(record *model.PluginRecord, spec *manifest.Manifest, packageDigest string, installedFiles []string) error {
	storedManifest, manifestErr := manifest.Parse([]byte(record.ManifestYAML))
	if (record.Status != model.PluginStatusUploaded && record.Status != model.PluginStatusPRFailed) ||
		manifestErr != nil ||
		storedManifest.ID != spec.ID ||
		storedManifest.Version != spec.Version ||
		record.Version != spec.Version ||
		record.PackageDigest != packageDigest ||
		!slices.Equal(record.InstalledFiles, installedFiles) {
		return domain.ErrPluginIDConflict
	}
	return nil
}

func installFiles(inspected *plugininstall.InspectedZip, manifestBytes []byte) (map[string][]byte, error) {
	files := make(map[string][]byte, len(inspected.BackendFiles)+len(inspected.FrontendFiles)+1)
	backendRoot := path.Join("backend/internal/plugins", inspected.Manifest.ID)
	for name, content := range inspected.BackendFiles {
		if err := plugininstall.ValidateRepoRelativePath(name); err != nil {
			return nil, err
		}
		target := path.Join(backendRoot, name)
		if err := plugininstall.ValidateRepoRelativePath(target); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(target, backendRoot+"/") {
			return nil, fmt.Errorf("unsafe backend plugin path: %q", name)
		}
		files[target] = append([]byte(nil), content...)
	}
	for name, content := range inspected.FrontendFiles {
		if err := plugininstall.ValidateRepoRelativePath(name); err != nil {
			return nil, err
		}
		target := path.Join("fronted/apps/web", name)
		if err := plugininstall.ValidateRepoRelativePath(target); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(target, "fronted/apps/web/") {
			return nil, fmt.Errorf("unsafe frontend plugin path: %q", name)
		}
		if !plugininstall.AllowedFrontendRel(inspected.Manifest.ID, name) {
			return nil, fmt.Errorf("frontend path is outside plugin directories: %q", name)
		}
		files[target] = append([]byte(nil), content...)
	}
	files[path.Join(backendRoot, "plugin.yaml")] = append([]byte(nil), manifestBytes...)
	return files, nil
}

func registeredPluginIDs(src []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "register.go", src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	prefix := backendModulePath + "/internal/plugins/"
	ids := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(importPath, prefix) {
			ids = append(ids, strings.TrimPrefix(importPath, prefix))
		}
	}
	return ids, nil
}

func resolveInstallRoot(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	if root := strings.TrimSpace(os.Getenv("PLUGIN_GIT_ROOT")); root != "" {
		return root, nil
	}
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, statErr := os.Stat(filepath.Join(current, "backend", "go.mod")); statErr == nil && !info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("repository root containing backend/go.mod not found")
		}
		current = parent
	}
}

func firstRoot(roots []string) string {
	if len(roots) == 0 {
		return ""
	}
	return roots[0]
}
