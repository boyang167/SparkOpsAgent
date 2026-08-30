package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	bundledkubernetes "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSeedBundledKubernetesPluginDoesNotOverwriteExistingRecord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin").Error)
	require.NoError(t, db.AutoMigrate(&pluginModel{}, &workspaceBindingModel{}, &uninstallReceiptModel{}))

	repo := NewStore(db)
	ctx := context.Background()
	installed := []string{"custom/kubernetes/plugin.go"}
	require.NoError(t, repo.SavePlugin(ctx, &model.PluginRecord{
		ID:             "kubernetes",
		Version:        "9.9.9",
		DisplayName:    "Custom Kubernetes",
		ManifestYAML:   "id: kubernetes",
		InstalledFiles: installed,
		Status:         model.PluginStatusUploaded,
		CreatedBy:      kernel.NewMemberID(),
	}))

	require.NoError(t, seedBundledKubernetesPlugin(db))
	found, err := repo.FindPlugin(ctx, "kubernetes")
	require.NoError(t, err)
	require.Equal(t, "9.9.9", found.Version)
	require.Equal(t, "Custom Kubernetes", found.DisplayName)
	require.Equal(t, model.PluginStatusUploaded, found.Status)
	require.Equal(t, installed, found.InstalledFiles)
}

func TestSeedBundledKubernetesPluginPersistsEmbeddedManifest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin").Error)
	require.NoError(t, db.AutoMigrate(&pluginModel{}, &workspaceBindingModel{}, &uninstallReceiptModel{}))

	require.NoError(t, seedBundledKubernetesPlugin(db))
	found, err := NewStore(db).FindPlugin(context.Background(), "kubernetes")
	require.NoError(t, err)
	require.Equal(t, bundledkubernetes.ManifestYAML, found.ManifestYAML)
}

func TestSeedBundledKubernetesPluginSkipsWhenUninstallReceiptExists(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin").Error)
	require.NoError(t, db.AutoMigrate(&pluginModel{}, &workspaceBindingModel{}, &uninstallReceiptModel{}))
	require.NoError(t, db.Create(&uninstallReceiptModel{
		PluginID:   "kubernetes",
		DeployedAt: time.Now().UTC(),
	}).Error)

	require.NoError(t, seedBundledKubernetesPlugin(db))
	_, err = NewStore(db).FindPlugin(context.Background(), "kubernetes")
	require.ErrorIs(t, err, domain.ErrPluginNotFound)
}
