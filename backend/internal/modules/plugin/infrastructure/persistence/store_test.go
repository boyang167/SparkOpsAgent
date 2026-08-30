package persistence_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openPluginStore(t *testing.T) (*gorm.DB, repository.PluginRepository) {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, persistence.AutoMigrate(db))
	return db, persistence.NewStore(db)
}

func TestAutoMigrateCreatesPluginSchemaTables(t *testing.T) {
	db, _ := openPluginStore(t)

	for _, table := range []string{"plugin.plugins", "plugin.workspace_bindings"} {
		var exists bool
		require.NoError(t, db.Raw("SELECT to_regclass(?) IS NOT NULL", table).Scan(&exists).Error)
		require.Truef(t, exists, "missing table %s", table)
	}
}

func TestStorePersistsPluginsAndWorkspaceBindings(t *testing.T) {
	db, repo := openPluginStore(t)
	ctx := context.Background()
	pluginID := fmt.Sprintf("example-%s", uuid.NewString())
	wsID := kernel.NewWorkspaceID()
	createdBy := kernel.NewMemberID()
	enabledBy := kernel.NewMemberID()
	enabledAt := time.Now().UTC().Truncate(time.Microsecond)
	plugin := &model.PluginRecord{
		ID:           pluginID,
		Version:      "0.1.0",
		DisplayName:  "Example Plugin",
		ManifestYAML: "id: example",
		Status:       model.PluginStatusUploaded,
		CreatedBy:    createdBy,
		CreatedAt:    enabledAt.Add(-time.Hour),
		UpdatedAt:    enabledAt.Add(-time.Hour),
	}
	t.Cleanup(func() {
		_ = repo.DeleteBinding(ctx, wsID, pluginID)
		_ = repo.DeletePlugin(ctx, pluginID)
	})

	require.NoError(t, repo.SavePlugin(ctx, plugin))
	found, err := repo.FindPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.Equal(t, pluginID, found.ID)
	require.Equal(t, "0.1.0", found.Version)
	require.Equal(t, "Example Plugin", found.DisplayName)
	require.Equal(t, model.PluginStatusUploaded, found.Status)
	require.Equal(t, createdBy, found.CreatedBy)

	require.NoError(t, found.MarkPROpen("https://example.test/pr/1"))
	require.NoError(t, repo.SavePlugin(ctx, found))
	reopened, err := repo.FindPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusPROpen, reopened.Status)
	require.Equal(t, "https://example.test/pr/1", reopened.PRURL)

	binding := &model.WorkspaceBinding{
		WorkspaceID: wsID,
		PluginID:    pluginID,
		Enabled:     true,
		EnabledBy:   enabledBy,
		EnabledAt:   enabledAt,
	}
	require.NoError(t, repo.SaveBinding(ctx, binding))

	foundBinding, err := repo.FindBinding(ctx, wsID, pluginID)
	require.NoError(t, err)
	require.Equal(t, wsID, foundBinding.WorkspaceID)
	require.Equal(t, pluginID, foundBinding.PluginID)
	require.True(t, foundBinding.Enabled)
	require.Equal(t, enabledBy, foundBinding.EnabledBy)
	require.Equal(t, enabledAt, foundBinding.EnabledAt)

	bindingsByPlugin, err := repo.ListBindingsByPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.Len(t, bindingsByPlugin, 1)
	require.Equal(t, wsID, bindingsByPlugin[0].WorkspaceID)

	enabledByWorkspace, err := repo.ListEnabledByWorkspace(ctx, wsID)
	require.NoError(t, err)
	require.Contains(t, pluginIDs(enabledByWorkspace), pluginID)

	require.NoError(t, repo.DeleteBinding(ctx, wsID, pluginID))
	_, err = repo.FindBinding(ctx, wsID, pluginID)
	require.ErrorIs(t, err, domain.ErrPluginBindingNotFound)

	require.NoError(t, repo.DeletePlugin(ctx, pluginID))
	_, err = repo.FindPlugin(ctx, pluginID)
	require.ErrorIs(t, err, domain.ErrPluginNotFound)

	var remaining int64
	require.NoError(t, db.Table("plugin.plugins").Where("id = ?", pluginID).Count(&remaining).Error)
	require.Zero(t, remaining)
}

func pluginIDs(bindings []*model.WorkspaceBinding) []string {
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, binding.PluginID)
	}
	return out
}
