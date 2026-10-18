package acl

import (
	"context"
	"errors"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

const bundledPluginCreatorID = "00000000-0000-0000-0000-000000000000"

var bundledPluginIDs = []string{"kubernetes"}

type bundledPluginRow struct {
	ID           string `gorm:"column:id;primaryKey"`
	ManifestYAML string `gorm:"column:manifest_yaml"`
	Version      string
	DisplayName  string `gorm:"column:display_name"`
	Status       string
	CreatedBy    string `gorm:"column:created_by"`
}

func (bundledPluginRow) TableName() string { return "plugin.plugins" }

type uninstallReceiptRow struct {
	PluginID string `gorm:"column:plugin_id;primaryKey"`
}

func (uninstallReceiptRow) TableName() string { return "plugin.uninstall_receipts" }

type enabledBindingRow struct {
	WorkspaceID string `gorm:"column:workspace_id"`
}

func (enabledBindingRow) TableName() string { return "plugin.workspace_bindings" }

// SyncBundledPlugins updates bundled plugin manifests in the database and rebinds
// governance menus for enabled workspaces when the embedded manifest changes.
func SyncBundledPlugins(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	binder := NewGovernanceBinder(db)
	for _, pluginID := range bundledPluginIDs {
		if err := syncBundledPlugin(ctx, db, binder, pluginID); err != nil {
			return err
		}
	}
	return nil
}

func syncBundledPlugin(ctx context.Context, db *gorm.DB, binder *GovernanceBinder, pluginID string) error {
	descriptor, ok := sparkplugin.Bundled(pluginID)
	if !ok || descriptor.ManifestYAML == "" {
		return nil
	}
	if db.Migrator().HasTable("plugin.uninstall_receipts") {
		var receipt uninstallReceiptRow
		err := db.WithContext(ctx).Where("plugin_id = ?", pluginID).First(&receipt).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	var row bundledPluginRow
	err := db.WithContext(ctx).Where("id = ?", pluginID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !isBundledManagedPlugin(row) {
		return nil
	}
	manifestYAML := descriptor.ManifestYAML
	if row.ManifestYAML != manifestYAML {
		parsed, err := manifest.Parse([]byte(manifestYAML))
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		result := db.WithContext(ctx).Model(&bundledPluginRow{}).
			Where("id = ? AND created_by = ? AND status = ?", pluginID, bundledPluginCreatorID, string(model.PluginStatusDeployed)).
			Updates(map[string]any{
				"manifest_yaml": manifestYAML,
				"version":       parsed.Version,
				"display_name":  parsed.DisplayName,
				"updated_at":    now,
			})
		if result.Error != nil {
			return result.Error
		}
	}
	return rebindEnabledWorkspaces(ctx, db, binder, pluginID, manifestYAML)
}

func rebindEnabledWorkspaces(ctx context.Context, db *gorm.DB, binder *GovernanceBinder, pluginID, manifestYAML string) error {
	spec, err := manifest.Parse([]byte(manifestYAML))
	if err != nil {
		return err
	}
	var bindings []enabledBindingRow
	if err := db.WithContext(ctx).
		Where("plugin_id = ? AND enabled = ?", pluginID, true).
		Find(&bindings).Error; err != nil {
		return err
	}
	for _, binding := range bindings {
		workspaceID, err := kernel.ParseWorkspaceID(binding.WorkspaceID)
		if err != nil {
			return err
		}
		if err := binder.Bind(ctx, workspaceID, spec); err != nil {
			return err
		}
	}
	return nil
}

func isBundledManagedPlugin(row bundledPluginRow) bool {
	return row.CreatedBy == bundledPluginCreatorID && row.Status == string(model.PluginStatusDeployed)
}
