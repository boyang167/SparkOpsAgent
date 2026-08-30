package command_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/command"
	plugininstall "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
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
	plugins  map[string]*model.PluginRecord
	bindings map[string][]*model.WorkspaceBinding
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		plugins:  make(map[string]*model.PluginRecord),
		bindings: make(map[string][]*model.WorkspaceBinding),
	}
}

func (r *memoryRepository) SavePlugin(_ context.Context, plugin *model.PluginRecord) error {
	copy := *plugin
	r.plugins[plugin.ID] = &copy
	return nil
}

func (r *memoryRepository) FindPlugin(_ context.Context, id string) (*model.PluginRecord, error) {
	plugin, ok := r.plugins[id]
	if !ok {
		return nil, domain.ErrPluginNotFound
	}
	copy := *plugin
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

func TestInstallPublishesFilesWithoutWritingCurrentTree(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	publisher := &plugininstall.MemoryPublisher{}
	handler := command.NewInstallHandler(repo, publisher, root)

	record, err := handler.Handle(context.Background(), validZip(t), kernel.NewMemberID())

	require.NoError(t, err)
	require.Equal(t, model.PluginStatusPROpen, record.Status)
	require.Equal(t, "plugin/install-example", publisher.Last.Branch)
	require.Equal(t, pluginGo, string(publisher.Last.Files["backend/internal/plugins/example/plugin.go"]))
	require.Equal(t, "export default function Page(){return null}", string(publisher.Last.Files["fronted/apps/web/app/(platform)/plugins/example/page.tsx"]))
	require.Equal(t, "export const helper = true", string(publisher.Last.Files["fronted/apps/web/src/plugins/example/helper.ts"]))
	require.Contains(t, string(publisher.Last.Files["backend/internal/plugins/example/plugin.yaml"]), "id: example")
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

func TestUninstallRejectsEnabledBinding(t *testing.T) {
	root := makeInstallRoot(t)
	repo := newMemoryRepository()
	repo.plugins["example"] = &model.PluginRecord{ID: "example", Status: model.PluginStatusDeployed, ManifestYAML: manifestYAML}
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
	repo.plugins["example"] = &model.PluginRecord{ID: "example", Status: model.PluginStatusMerged, ManifestYAML: manifestYAML}
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
	repo.plugins["example"] = &model.PluginRecord{ID: "example", Status: model.PluginStatusDeployed, ManifestYAML: manifestYAML}
	publisher := &plugininstall.MemoryPublisher{}
	handler := command.NewUninstallHandler(repo, publisher, root)

	record, err := handler.Handle(context.Background(), "example", kernel.NewMemberID())

	require.NoError(t, err)
	require.Equal(t, model.PluginStatusUninstalling, record.Status)
	require.Equal(t, "plugin/uninstall-example", publisher.Last.Branch)
	require.Contains(t, publisher.Last.Delete, "backend/internal/plugins/example")
	require.Contains(t, publisher.Last.Delete, "fronted/apps/web/app/(platform)/plugins/example/page.tsx")
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
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	files := map[string]string{
		"example/plugin.yaml":                                      manifestYAML,
		"example/backend/plugin.go":                                pluginGo,
		"example/frontend/app/(platform)/plugins/example/page.tsx": "export default function Page(){return null}",
		"example/frontend/src/plugins/example/helper.ts":           "export const helper = true",
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

func requireDomainCode(t *testing.T, err error, code string) {
	t.Helper()
	domainErr, ok := kernel.IsDomainError(err)
	require.True(t, ok, "error %v is not a domain error", err)
	require.Equal(t, code, domainErr.Code())
}
