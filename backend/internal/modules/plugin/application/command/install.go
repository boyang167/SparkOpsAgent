package command

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	plugininstall "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
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
	if _, err := h.repo.FindPlugin(ctx, inspected.Manifest.ID); err == nil {
		return nil, domain.ErrPluginIDConflict
	} else if !errors.Is(err, domain.ErrPluginNotFound) {
		return nil, err
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

	manifestBytes, err := yaml.Marshal(inspected.Manifest)
	if err != nil {
		return nil, err
	}
	record := &model.PluginRecord{
		ID:           inspected.Manifest.ID,
		Version:      inspected.Manifest.Version,
		DisplayName:  inspected.Manifest.DisplayName,
		ManifestYAML: string(manifestBytes),
		Status:       model.PluginStatusUploaded,
		CreatedBy:    actor,
	}
	if err := h.repo.SavePlugin(ctx, record); err != nil {
		return nil, err
	}

	files := installFiles(inspected, manifestBytes)
	files[registerGoPath] = nextRegister
	prURL, publishErr := h.publisher.Publish(ctx, plugininstall.PublishRequest{
		Branch:  "plugin/install-" + record.ID,
		Message: "Install plugin " + record.ID,
		Files:   files,
	})
	if publishErr != nil {
		if err := record.MarkPRFailed(); err != nil {
			return record, errors.Join(publishErr, err)
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

func installFiles(inspected *plugininstall.InspectedZip, manifestBytes []byte) map[string][]byte {
	files := make(map[string][]byte, len(inspected.BackendFiles)+len(inspected.FrontendFiles)+1)
	backendRoot := path.Join("backend/internal/plugins", inspected.Manifest.ID)
	for name, content := range inspected.BackendFiles {
		files[path.Join(backendRoot, name)] = append([]byte(nil), content...)
	}
	for name, content := range inspected.FrontendFiles {
		files[path.Join("fronted/apps/web", name)] = append([]byte(nil), content...)
	}
	files[path.Join(backendRoot, "plugin.yaml")] = append([]byte(nil), manifestBytes...)
	return files
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
