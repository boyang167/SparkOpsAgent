package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS plugin").Error; err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(&pluginModel{}, &workspaceBindingModel{}, &uninstallReceiptModel{}); err != nil {
		return err
	}
	if db.Dialector.Name() == "postgres" {
		return db.Exec(`
			DO $$
			BEGIN
				IF NOT EXISTS (
					SELECT 1
					FROM pg_constraint c
					JOIN pg_class rel ON rel.oid = c.conrelid
					JOIN pg_namespace ns ON ns.oid = rel.relnamespace
					WHERE ns.nspname = 'plugin'
					  AND rel.relname = 'workspace_bindings'
					  AND c.conname = 'fk_plugin_workspace_bindings_plugin'
					  AND c.contype = 'f'
				) THEN
					ALTER TABLE plugin.workspace_bindings
						ADD CONSTRAINT fk_plugin_workspace_bindings_plugin
						FOREIGN KEY (plugin_id)
						REFERENCES plugin.plugins(id);
				END IF;
			END
			$$;
		`).Error
	}
	return nil
}

type pluginModel struct {
	ID                 string    `gorm:"column:id;primaryKey;type:text"`
	Version            string    `gorm:"type:text;not null"`
	DisplayName        string    `gorm:"type:text;not null"`
	ManifestYAML       string    `gorm:"type:text;not null"`
	PackageDigest      string    `gorm:"column:package_digest;type:text;not null;default:''"`
	InstalledFilesJSON string    `gorm:"column:installed_files;type:jsonb;not null;default:'[]'"`
	Status             string    `gorm:"type:text;not null"`
	PRURL              string    `gorm:"column:pr_url;type:text;not null;default:''"`
	CreatedBy          string    `gorm:"type:uuid;not null"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (pluginModel) TableName() string { return "plugin.plugins" }

type workspaceBindingModel struct {
	WorkspaceID string  `gorm:"column:workspace_id;primaryKey;type:uuid;not null"`
	PluginID    string  `gorm:"column:plugin_id;primaryKey;type:text;not null"`
	Enabled     bool    `gorm:"not null;default:false"`
	EnabledBy   *string `gorm:"column:enabled_by;type:uuid"`
	EnabledAt   *time.Time
}

func (workspaceBindingModel) TableName() string { return "plugin.workspace_bindings" }

type uninstallReceiptModel struct {
	PluginID   string    `gorm:"column:plugin_id;primaryKey;type:text"`
	DeployedAt time.Time `gorm:"column:deployed_at;not null"`
}

func (uninstallReceiptModel) TableName() string { return "plugin.uninstall_receipts" }

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) SavePlugin(ctx context.Context, plugin *model.PluginRecord) error {
	installedFiles, err := json.Marshal(plugin.InstalledFiles)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if plugin.CreatedAt.IsZero() {
		plugin.CreatedAt = now
	}
	plugin.UpdatedAt = now
	row := pluginModel{
		ID:                 plugin.ID,
		Version:            plugin.Version,
		DisplayName:        plugin.DisplayName,
		ManifestYAML:       plugin.ManifestYAML,
		PackageDigest:      plugin.PackageDigest,
		InstalledFilesJSON: string(installedFiles),
		Status:             string(plugin.Status),
		PRURL:              plugin.PRURL,
		CreatedBy:          plugin.CreatedBy.String(),
		CreatedAt:          plugin.CreatedAt,
		UpdatedAt:          plugin.UpdatedAt,
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *Store) FindPlugin(ctx context.Context, id string) (*model.PluginRecord, error) {
	var row pluginModel
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPluginNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToPlugin(row)
}

func (s *Store) ListPlugins(ctx context.Context) ([]*model.PluginRecord, error) {
	var rows []pluginModel
	if err := s.db.WithContext(ctx).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.PluginRecord, 0, len(rows))
	for _, row := range rows {
		plugin, err := rowToPlugin(row)
		if err != nil {
			return nil, err
		}
		out = append(out, plugin)
	}
	return out, nil
}

func (s *Store) DeletePlugin(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plugin_id = ?", id).Delete(&workspaceBindingModel{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ?", id).Delete(&pluginModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrPluginNotFound
		}
		return nil
	})
}

func (s *Store) CompleteUninstall(ctx context.Context, id string, deployedAt time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing pluginModel
		err := tx.Where("id = ?", id).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrPluginNotFound
		}
		if err != nil {
			return err
		}
		receipt := uninstallReceiptModel{PluginID: id, DeployedAt: deployedAt.UTC()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&receipt).Error; err != nil {
			return err
		}
		if err := tx.Where("plugin_id = ?", id).Delete(&workspaceBindingModel{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ?", id).Delete(&pluginModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrPluginNotFound
		}
		return nil
	})
}

func (s *Store) HasUninstallReceipt(ctx context.Context, id string) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&uninstallReceiptModel{}).
		Where("plugin_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) SaveBinding(ctx context.Context, binding *model.WorkspaceBinding) error {
	row := workspaceBindingModel{
		WorkspaceID: binding.WorkspaceID.String(),
		PluginID:    binding.PluginID,
		Enabled:     binding.Enabled,
	}
	if binding.EnabledBy.UUID != uuid.Nil {
		enabledBy := binding.EnabledBy.String()
		row.EnabledBy = &enabledBy
	}
	if !binding.EnabledAt.IsZero() {
		enabledAt := binding.EnabledAt.UTC()
		row.EnabledAt = &enabledAt
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *Store) FindBinding(ctx context.Context, workspaceID kernel.WorkspaceID, pluginID string) (*model.WorkspaceBinding, error) {
	var row workspaceBindingModel
	err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND plugin_id = ?", workspaceID.String(), pluginID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPluginBindingNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToBinding(row)
}

func (s *Store) ListBindingsByPlugin(ctx context.Context, pluginID string) ([]*model.WorkspaceBinding, error) {
	var rows []workspaceBindingModel
	if err := s.db.WithContext(ctx).Where("plugin_id = ?", pluginID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToBindings(rows)
}

func (s *Store) ListEnabledByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.WorkspaceBinding, error) {
	var rows []workspaceBindingModel
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND enabled = ?", workspaceID.String(), true).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToBindings(rows)
}

func (s *Store) DeleteBinding(ctx context.Context, workspaceID kernel.WorkspaceID, pluginID string) error {
	result := s.db.WithContext(ctx).
		Where("workspace_id = ? AND plugin_id = ?", workspaceID.String(), pluginID).
		Delete(&workspaceBindingModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrPluginBindingNotFound
	}
	return nil
}

func rowToPlugin(row pluginModel) (*model.PluginRecord, error) {
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, err
	}
	var installedFiles []string
	if err := json.Unmarshal([]byte(row.InstalledFilesJSON), &installedFiles); err != nil {
		return nil, err
	}
	if installedFiles == nil {
		installedFiles = []string{}
	}
	return &model.PluginRecord{
		ID:             row.ID,
		Version:        row.Version,
		DisplayName:    row.DisplayName,
		ManifestYAML:   row.ManifestYAML,
		PackageDigest:  row.PackageDigest,
		InstalledFiles: installedFiles,
		Status:         model.PluginStatus(row.Status),
		PRURL:          row.PRURL,
		CreatedBy:      createdBy,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}

func rowToBinding(row workspaceBindingModel) (*model.WorkspaceBinding, error) {
	workspaceID, err := kernel.ParseWorkspaceID(row.WorkspaceID)
	if err != nil {
		return nil, err
	}
	binding := &model.WorkspaceBinding{
		WorkspaceID: workspaceID,
		PluginID:    row.PluginID,
		Enabled:     row.Enabled,
	}
	if row.EnabledBy != nil && *row.EnabledBy != "" {
		enabledBy, err := kernel.ParseMemberID(*row.EnabledBy)
		if err != nil {
			return nil, err
		}
		binding.EnabledBy = enabledBy
	}
	if row.EnabledAt != nil {
		binding.EnabledAt = row.EnabledAt.UTC()
	}
	return binding, nil
}

func rowsToBindings(rows []workspaceBindingModel) ([]*model.WorkspaceBinding, error) {
	out := make([]*model.WorkspaceBinding, 0, len(rows))
	for _, row := range rows {
		binding, err := rowToBinding(row)
		if err != nil {
			return nil, err
		}
		out = append(out, binding)
	}
	return out, nil
}

var _ repository.PluginRepository = (*Store)(nil)
