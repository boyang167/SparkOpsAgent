package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS plugin").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&pluginModel{}, &workspaceBindingModel{})
}

type pluginModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	Version      string
	DisplayName  string
	ManifestYAML string `gorm:"type:text"`
	Status       string
	PRURL        string `gorm:"column:pr_url;default:'';not null"`
	CreatedBy    string `gorm:"type:uuid;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (pluginModel) TableName() string { return "plugin.plugins" }

type workspaceBindingModel struct {
	WorkspaceID string `gorm:"column:workspace_id;primaryKey;type:uuid"`
	PluginID    string `gorm:"column:plugin_id;primaryKey"`
	Enabled     bool
	EnabledBy   *string `gorm:"column:enabled_by;type:uuid"`
	EnabledAt   *time.Time
}

func (workspaceBindingModel) TableName() string { return "plugin.workspace_bindings" }

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) SavePlugin(ctx context.Context, plugin *model.PluginRecord) error {
	now := time.Now().UTC()
	if plugin.CreatedAt.IsZero() {
		plugin.CreatedAt = now
	}
	plugin.UpdatedAt = now
	row := pluginModel{
		ID:           plugin.ID,
		Version:      plugin.Version,
		DisplayName:  plugin.DisplayName,
		ManifestYAML: plugin.ManifestYAML,
		Status:       string(plugin.Status),
		PRURL:        plugin.PRURL,
		CreatedBy:    plugin.CreatedBy.String(),
		CreatedAt:    plugin.CreatedAt,
		UpdatedAt:    plugin.UpdatedAt,
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
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&pluginModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrPluginNotFound
	}
	return nil
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
	return &model.PluginRecord{
		ID:           row.ID,
		Version:      row.Version,
		DisplayName:  row.DisplayName,
		ManifestYAML: row.ManifestYAML,
		Status:       model.PluginStatus(row.Status),
		PRURL:        row.PRURL,
		CreatedBy:    createdBy,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
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
