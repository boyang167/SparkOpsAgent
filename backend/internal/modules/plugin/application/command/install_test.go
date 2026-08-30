package command_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/command"
	plugininstall "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const manifestYAML = `id: example
version: "0.1.0"
display_name: Example Plugin
route_prefix: /api/v1/plugins/example
entitlement: plugin:example
menus:
  - title: Example
    path: /plugins/example
    component: plugins/example/page
apis:
  - method: GET
    path: /api/v1/plugins/example/ping
frontend:
  pages:
    - app/(platform)/plugins/example/page.tsx
`

const pluginGo = `package example

import "github.com/gin-gonic/gin"

type plugin struct{}

func (p *plugin) Register(engine *gin.Engine) {}
`

type memoryRepository struct {
	plugins    map[string]*model.PluginRecord
	bindings   map[string][]*model.WorkspaceBinding
	saveCalls  int
	failSaveAt map[int]error
	receipts   map[string]time.Time
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		plugins:  make(map[string]*model.PluginRecord),
		bindings: make(map[string][]*model.WorkspaceBinding),
		receipts: make(map[string]time.Time),
	}
}

func (r *memoryRepository) SavePlugin(_ context.Context, plugin *model.PluginRecord) error {
	r.saveCalls++
	if err := r.failSaveAt[r.saveCalls]; err != nil {
		return err
	}
	if existing, ok := r.plugins[plugin.ID]; ok {
		if existing.PackageDigest != "" && plugin.PackageDigest != "" && existing.PackageDigest != plugin.PackageDigest {
			return domain.ErrPluginIDConflict
		}
	}
	copy := *plugin
	copy.InstalledFiles = append([]string(nil), plugin.InstalledFiles...)
	r.plugins[plugin.ID] = &copy
	return nil
}

func (r *memoryRepository) FindPlugin(_ context.Context, id string) (*model.PluginRecord, error) {
	plugin, ok := r.plugins[id]
	if !ok {
		return nil, domain.ErrPluginNotFound
	}
	copy := *plugin
	copy.InstalledFiles = append([]string(nil), plugin.InstalledFiles...)
	return &copy, nil
}

func (r *memoryRepository) ListPlugins(_ context.Context) ([]*model.PluginRecord, error) {
	out := make([]*model.PluginRecord, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		copy := *plugin
		out = append(out, &copy)
	}
	return out, nil
}

func (r *memoryRepository) DeletePlugin(_ context.Context, id string) error {
	if _, ok := r.plugins[id]; !ok {
		return domain.ErrPluginNotFound
	}
	delete(r.plugins, id)
	return nil
}

func (r *memoryRepository) CompleteUninstall(_ context.Context, id string, deployedAt time.Time) error {
	if _, ok := r.plugins[id]; !ok {
		return domain.ErrPluginNotFound
	}
	r.receipts[id] = deployedAt
	delete(r.bindings, id)
	delete(r.plugins, id)
	return nil
}

func (r *memoryRepository) HasUninstallReceipt(_ context.Context, id string) (bool, error) {
	_, ok := r.receipts[id]
	return ok, nil
}

func (r *memoryRepository) SaveBinding(_ context.Context, binding *model.WorkspaceBinding) error {
	r.bindings[binding.PluginID] = append(r.bindings[binding.PluginID], binding)
	return nil
}

func (r *memoryRepository) FindBinding(context.Context, kernel.WorkspaceID, string) (*model.WorkspaceBinding, error) {
	return nil, domain.ErrPluginBindingNotFound
}

func (r *memoryRepository) ListBindingsByPlugin(_ context.Context, pluginID string) ([]*model.WorkspaceBinding, error) {
	return r.bindings[pluginID], nil
}

func (r *memoryRepository) ListEnabledByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.WorkspaceBinding, error) {
	return nil, nil
}

func (r *memoryRepository) DeleteBinding(context.Context, kernel.WorkspaceID, string) error {
	return nil
}

type failingPublisher struct{ err error }

func (p failingPublisher) Publish(context.Context, plugininstall.PublishRequest) (string, error) {
	return "", p.err
}

func (p failingPublisher) LookupPullRequest(context.Context, string) (plugininstall.PullRequestView, error) {
	return plugininstall.PullRequestView{}, plugininstall.ErrPullRequestNotFound
}

func TestInstallPublishesFilesWithoutWritingCurrentTree(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	publisher := &plugininstall.MemoryPublisher{}
	handler := command.NewInstallHandler(repo, publisher, root)
	raw := validZip(t)

	record, err := handler.Handle(context.Background(), raw, kernel.NewMemberID())

	require.NoError(t, err)
	require.Equal(t, model.PluginStatusPROpen, record.Status)
	require.Equal(t, packageDigest(raw), record.PackageDigest)
	require.Equal(t, "plugin/install-example", publisher.Last.Branch)
	require.Equal(t, pluginGo, string(publisher.Last.Files["backend/internal/plugins/example/plugin.go"]))
	require.Equal(t, "export default function Page(){return null}", string(publisher.Last.Files["fronted/apps/web/app/(platform)/plugins/example/page.tsx"]))
	require.Equal(t, "export const helper = true", string(publisher.Last.Files["fronted/apps/web/src/plugins/example/helper.ts"]))
	require.Contains(t, string(publisher.Last.Files["backend/internal/plugins/example/plugin.yaml"]), "id: example")
	require.ElementsMatch(t, []string{
		"backend/internal/plugins/example/plugin.go",
		"backend/internal/plugins/example/plugin.yaml",
		"fronted/apps/web/app/(platform)/plugins/example/page.tsx",
		"fronted/apps/web/src/plugins/example/helper.ts",
	}, record.InstalledFiles)
	register := string(publisher.Last.Files["backend/internal/plugins/register.go"])
	require.Contains(t, register, `/internal/plugins/example"`)
	_, err = os.Stat(filepath.Join(root, "backend", "internal", "plugins", "example"))
	require.ErrorIs(t, err, os.ErrNotExist)
	currentRegister, err := os.ReadFile(filepath.Join(root, "backend", "internal", "plugins", "register.go"))
	require.NoError(t, err)
	require.NotContains(t, string(currentRegister), `/internal/plugins/example"`)
}

func TestInstallRejectsExistingPluginID(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	publisher := &plugininstall.MemoryPublisher{}
	handler := command.NewInstallHandler(repo, publisher, root)
	_, err := handler.Handle(context.Background(), validZip(t), kernel.NewMemberID())
	require.NoError(t, err)

	_, err = handler.Handle(context.Background(), validZip(t), kernel.NewMemberID())

	requireDomainCode(t, err, "PLUGIN_ID_CONFLICT")
}

func TestInstallPersistsPRFailedWhenPublishFails(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	publishErr := errors.New("push failed")
	handler := command.NewInstallHandler(repo, failingPublisher{err: publishErr}, root)

	record, err := handler.Handle(context.Background(), validZip(t), kernel.NewMemberID())

	require.ErrorIs(t, err, publishErr)
	require.Equal(t, model.PluginStatusPRFailed, record.Status)
	saved, findErr := repo.FindPlugin(context.Background(), "example")
	require.NoError(t, findErr)
	require.Equal(t, model.PluginStatusPRFailed, saved.Status)
}

func TestInstallRecoversPublishedPRWhenStatusSaveFailed(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	saveErr := errors.New("save status failed")
	repo.failSaveAt = map[int]error{2: saveErr}
	publisher := &plugininstall.MemoryPublisher{}
	handler := command.NewInstallHandler(repo, publisher, root)
	raw := validZip(t)

	record, err := handler.Handle(context.Background(), raw, kernel.NewMemberID())
	require.ErrorIs(t, err, saveErr)
	require.Equal(t, model.PluginStatusPROpen, record.Status)
	require.Equal(t, 1, publisher.PublishCount)
	saved, findErr := repo.FindPlugin(context.Background(), "example")
	require.NoError(t, findErr)
	require.Equal(t, model.PluginStatusUploaded, saved.Status)

	repo.failSaveAt = nil
	record, err = handler.Handle(context.Background(), raw, kernel.NewMemberID())

	require.NoError(t, err)
	require.Equal(t, model.PluginStatusPROpen, record.Status)
	require.Equal(t, "https://example.local/pr/1", record.PRURL)
	require.Equal(t, 1, publisher.PublishCount)
}

func TestInstallRetriesPRFailedWhenNoPullRequestExists(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	raw := validZip(t)
	repo.plugins["example"] = retryRecord(t, raw, model.PluginStatusPRFailed)
	publisher := &plugininstall.MemoryPublisher{}

	record, err := command.NewInstallHandler(repo, publisher, root).Handle(context.Background(), raw, kernel.NewMemberID())

	require.NoError(t, err)
	require.Equal(t, model.PluginStatusPROpen, record.Status)
	require.Equal(t, 1, publisher.PublishCount)
}

func TestInstallRejectsRetryWithDifferentManifestIDOrVersion(t *testing.T) {
	raw := validZip(t)
	tests := []struct {
		name     string
		version  string
		manifest string
	}{
		{name: "different version", version: "9.9.9", manifest: normalizedManifestFrom(t, raw)},
		{name: "different manifest id", version: "0.1.0", manifest: strings.Replace(normalizedManifestFrom(t, raw), "id: example", "id: other", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := makeInstallRoot(t)
			repo := newMemoryRepository()
			repo.plugins["example"] = &model.PluginRecord{
				ID:             "example",
				Version:        tt.version,
				ManifestYAML:   tt.manifest,
				PackageDigest:  packageDigest(raw),
				InstalledFiles: installedFiles(),
				Status:         model.PluginStatusUploaded,
			}

			_, err := command.NewInstallHandler(repo, &plugininstall.MemoryPublisher{}, root).
				Handle(context.Background(), raw, kernel.NewMemberID())

			requireDomainCode(t, err, "PLUGIN_ID_CONFLICT")
		})
	}
}

func TestInstallRejectsSameManifestVersionWithDifferentPackageBytes(t *testing.T) {
	root := makeInstallRoot(t)
	original := validZipWithHelper(t, "export const helper = true")
	retry := validZipWithHelper(t, "export const helper = false")
	repo := newMemoryRepository()
	repo.plugins["example"] = retryRecord(t, original, model.PluginStatusUploaded)
	originalFiles := append([]string(nil), repo.plugins["example"].InstalledFiles...)

	_, err := command.NewInstallHandler(repo, &plugininstall.MemoryPublisher{}, root).
		Handle(context.Background(), retry, kernel.NewMemberID())

	requireDomainCode(t, err, "PLUGIN_ID_CONFLICT")
	require.Equal(t, originalFiles, repo.plugins["example"].InstalledFiles)
}

func TestInstallRejectsRetryWhenPersistedInstalledFilesDiffer(t *testing.T) {
	root := makeInstallRoot(t)
	raw := validZip(t)
	repo := newMemoryRepository()
	record := retryRecord(t, raw, model.PluginStatusUploaded)
	record.InstalledFiles = []string{"backend/internal/plugins/example/original.go"}
	repo.plugins["example"] = record

	_, err := command.NewInstallHandler(repo, &plugininstall.MemoryPublisher{}, root).
		Handle(context.Background(), raw, kernel.NewMemberID())

	requireDomainCode(t, err, "PLUGIN_ID_CONFLICT")
	require.Equal(t, []string{"backend/internal/plugins/example/original.go"}, repo.plugins["example"].InstalledFiles)
}

func TestInstallDoesNotPublishWhenPullRequestLookupFails(t *testing.T) {
	root := makeInstallRoot(t)
	raw := validZip(t)
	repo := newMemoryRepository()
	repo.plugins["example"] = retryRecord(t, raw, model.PluginStatusUploaded)
	lookupErr := errors.New("github unavailable")
	publisher := &plugininstall.MemoryPublisher{LookupErr: lookupErr}

	_, err := command.NewInstallHandler(repo, publisher, root).
		Handle(context.Background(), raw, kernel.NewMemberID())

	require.ErrorIs(t, err, lookupErr)
	require.Zero(t, publisher.PublishCount)
}

func TestInstallDoesNotReuseOrReplaceClosedPullRequest(t *testing.T) {
	root := makeInstallRoot(t)
	raw := validZip(t)
	repo := newMemoryRepository()
	repo.plugins["example"] = retryRecord(t, raw, model.PluginStatusUploaded)
	branch := "plugin/install-example"
	publisher := &plugininstall.MemoryPublisher{PullRequests: map[string]plugininstall.PullRequestView{
		branch: {URL: "https://example.test/pr/1", State: "CLOSED", HeadRefName: branch},
	}}

	_, err := command.NewInstallHandler(repo, publisher, root).
		Handle(context.Background(), raw, kernel.NewMemberID())

	require.ErrorIs(t, err, plugininstall.ErrPullRequestNotRecoverable)
	require.Zero(t, publisher.PublishCount)
}

func TestUninstallRejectsEnabledBinding(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	repo.plugins["example"] = &model.PluginRecord{ID: "example", Status: model.PluginStatusDeployed, ManifestYAML: manifestYAML, InstalledFiles: installedFiles()}
	repo.bindings["example"] = []*model.WorkspaceBinding{{PluginID: "example", Enabled: true}}
	publisher := &plugininstall.MemoryPublisher{}
	handler := command.NewUninstallHandler(repo, publisher, root)

	_, err := handler.Handle(context.Background(), "example", kernel.NewMemberID())

	requireDomainCode(t, err, "PLUGIN_IN_USE")
	require.Empty(t, publisher.Last.Branch)
}

func TestUninstallRequiresDeployedPlugin(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	repo.plugins["example"] = &model.PluginRecord{ID: "example", Status: model.PluginStatusMerged, ManifestYAML: manifestYAML, InstalledFiles: installedFiles()}
	handler := command.NewUninstallHandler(repo, &plugininstall.MemoryPublisher{}, root)

	_, err := handler.Handle(context.Background(), "example", kernel.NewMemberID())

	requireDomainCode(t, err, "PLUGIN_NOT_DEPLOYED")
}

func TestUninstallPublishesDeletionAndWaitsForDeploymentToDeleteRecord(t *testing.T) {
	root := makeInstallRoot(t)
	registerPath := filepath.Join(root, "backend", "internal", "plugins", "register.go")
	require.NoError(t, os.WriteFile(registerPath, []byte(`package plugins

import (
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins/example"
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins/keep"
)
`), 0o644))
	repo := newMemoryRepository()
	repo.plugins["example"] = &model.PluginRecord{ID: "example", Status: model.PluginStatusDeployed, ManifestYAML: manifestYAML, InstalledFiles: installedFiles()}
	publisher := &plugininstall.MemoryPublisher{}
	handler := command.NewUninstallHandler(repo, publisher, root)

	record, err := handler.Handle(context.Background(), "example", kernel.NewMemberID())

	require.NoError(t, err)
	require.Equal(t, model.PluginStatusUninstalling, record.Status)
	require.Equal(t, "plugin/uninstall-example", publisher.Last.Branch)
	require.ElementsMatch(t, append(installedFiles(), "backend/internal/plugins/example"), publisher.Last.Delete)
	register := string(publisher.Last.Files["backend/internal/plugins/register.go"])
	require.NotContains(t, register, `/internal/plugins/example"`)
	require.Contains(t, register, `/internal/plugins/keep"`)
	_, err = repo.FindPlugin(context.Background(), "example")
	require.NoError(t, err)

	merged := command.NewMarkMergedHandler(repo)
	record, err = merged.Handle(context.Background(), "example")
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusUninstallMerged, record.Status)

	deployed := command.NewMarkDeployedHandler(repo)
	record, err = deployed.Handle(context.Background(), "example")
	require.NoError(t, err)
	require.Nil(t, record)
	_, err = repo.FindPlugin(context.Background(), "example")
	require.ErrorIs(t, err, domain.ErrPluginNotFound)

	record, err = deployed.Handle(context.Background(), "example")
	require.NoError(t, err)
	require.Nil(t, record)
}

func TestUninstallRejectsUnsafePersistedInstalledPath(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	repo.plugins["example"] = &model.PluginRecord{
		ID:             "example",
		Status:         model.PluginStatusDeployed,
		InstalledFiles: []string{"../READEMD.md"},
	}
	publisher := &plugininstall.MemoryPublisher{}

	_, err := command.NewUninstallHandler(repo, publisher, root).
		Handle(context.Background(), "example", kernel.NewMemberID())

	require.Error(t, err)
	require.Zero(t, publisher.PublishCount)
}

func TestUninstallRejectsUnsafePersistedPluginID(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	repo.plugins["../.."] = &model.PluginRecord{
		ID:     "../..",
		Status: model.PluginStatusDeployed,
	}
	publisher := &plugininstall.MemoryPublisher{}

	_, err := command.NewUninstallHandler(repo, publisher, root).
		Handle(context.Background(), "../..", kernel.NewMemberID())

	require.Error(t, err)
	require.Zero(t, publisher.PublishCount)
}

func TestUninstallRecoversPublishedPRWhenStatusSaveFailed(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	repo.plugins["example"] = &model.PluginRecord{
		ID:             "example",
		Status:         model.PluginStatusDeployed,
		InstalledFiles: installedFiles(),
	}
	saveErr := errors.New("save uninstall status failed")
	repo.failSaveAt = map[int]error{1: saveErr}
	publisher := &plugininstall.MemoryPublisher{}
	handler := command.NewUninstallHandler(repo, publisher, root)

	_, err := handler.Handle(context.Background(), "example", kernel.NewMemberID())
	require.ErrorIs(t, err, saveErr)
	require.Equal(t, 1, publisher.PublishCount)
	saved, findErr := repo.FindPlugin(context.Background(), "example")
	require.NoError(t, findErr)
	require.Equal(t, model.PluginStatusDeployed, saved.Status)

	repo.failSaveAt = nil
	record, err := handler.Handle(context.Background(), "example", kernel.NewMemberID())

	require.NoError(t, err)
	require.Equal(t, model.PluginStatusUninstalling, record.Status)
	require.Equal(t, 1, publisher.PublishCount)
}

func TestStatusCallbacksAreIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	repo.plugins["install"] = &model.PluginRecord{ID: "install", Status: model.PluginStatusPROpen}
	merged := command.NewMarkMergedHandler(repo)
	first, err := merged.Handle(ctx, "install")
	require.NoError(t, err)
	second, err := merged.Handle(ctx, "install")
	require.NoError(t, err)
	require.Equal(t, first.Status, second.Status)

	deployed := command.NewMarkDeployedHandler(repo)
	first, err = deployed.Handle(ctx, "install")
	require.NoError(t, err)
	second, err = deployed.Handle(ctx, "install")
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusDeployed, second.Status)

	repo.plugins["uninstall"] = &model.PluginRecord{ID: "uninstall", Status: model.PluginStatusUninstalling}
	first, err = merged.Handle(ctx, "uninstall")
	require.NoError(t, err)
	second, err = merged.Handle(ctx, "uninstall")
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusUninstallMerged, first.Status)
	require.Equal(t, first.Status, second.Status)
}

func TestStatusCallbacksStillRejectReverseTransitions(t *testing.T) {
	repo := newMemoryRepository()
	repo.plugins["example"] = &model.PluginRecord{ID: "example", Status: model.PluginStatusDeployed}

	_, err := command.NewMarkMergedHandler(repo).Handle(context.Background(), "example")

	require.ErrorIs(t, err, model.ErrInvalidPluginStatusTransition)
}

func TestDeployedCallbackRejectsUnknownPluginWithoutReceipt(t *testing.T) {
	repo := newMemoryRepository()

	_, err := command.NewMarkDeployedHandler(repo).Handle(context.Background(), "unknown")

	require.ErrorIs(t, err, domain.ErrPluginNotFound)
}

func makeInstallRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "backend", "internal", "plugins"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "backend", "go.mod"), []byte("module github.com/sparkops/sparkops-agent/backend\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "backend", "internal", "plugins", "register.go"), []byte("package plugins\n"), 0o644))
	return root
}

func validZip(t *testing.T) []byte {
	return validZipWithHelper(t, "export const helper = true")
}

func validZipWithHelper(t *testing.T, helper string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	files := map[string]string{
		"example/plugin.yaml":                                      manifestYAML,
		"example/backend/plugin.go":                                pluginGo,
		"example/frontend/app/(platform)/plugins/example/page.tsx": "export default function Page(){return null}",
		"example/frontend/src/plugins/example/helper.ts":           helper,
	}
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

func installedFiles() []string {
	return []string{
		"backend/internal/plugins/example/plugin.go",
		"backend/internal/plugins/example/plugin.yaml",
		"fronted/apps/web/app/(platform)/plugins/example/page.tsx",
		"fronted/apps/web/src/plugins/example/helper.ts",
	}
}

func normalizedManifestFrom(t *testing.T, rawZip []byte) string {
	t.Helper()
	inspected, err := plugininstall.InspectZip(rawZip)
	require.NoError(t, err)
	raw, err := yaml.Marshal(inspected.Manifest)
	require.NoError(t, err)
	return string(raw)
}

func packageDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func retryRecord(t *testing.T, raw []byte, status model.PluginStatus) *model.PluginRecord {
	t.Helper()
	return &model.PluginRecord{
		ID:             "example",
		Version:        "0.1.0",
		ManifestYAML:   normalizedManifestFrom(t, raw),
		PackageDigest:  packageDigest(raw),
		InstalledFiles: installedFiles(),
		Status:         status,
	}
}

func requireDomainCode(t *testing.T, err error, code string) {
	t.Helper()
	domainErr, ok := kernel.IsDomainError(err)
	require.True(t, ok, "error %v is not a domain error", err)
	require.Equal(t, code, domainErr.Code())
}
