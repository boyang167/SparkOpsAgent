package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"


	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"gorm.io/gorm/clause"
)

func AutoMigrate(db *gorm.DB) error {
	if postgres.ShouldRunAutoMigrate(db) {
		if db.Dialector.Name() == "postgres" {
			if err := db.Exec("CREATE SCHEMA IF NOT EXISTS plugin").Error; err != nil {
				return err
			}
		}
		if err := db.AutoMigrate(&pluginModel{}, &workspaceBindingModel{}, &uninstallReceiptModel{}); err != nil {
			return err
		}
		if db.Dialector.Name() == "postgres" {
			if err := db.Exec(`
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
			`).Error; err != nil {
				return err
			}
		}
	}
	return SeedBundledPlugins(db)
}

// SeedBundledPlugins ensures built-in plugin records exist even when SQL migrations
// already applied and GORM AutoMigrate is skipped.
func SeedBundledPlugins(db *gorm.DB) error {
	if err := seedBundledExamplePlugin(db); err != nil {
		return err
	}
	if err := seedBundledKubernetesPlugin(db); err != nil {
		return err
	}
	return pruneRemovedPluginMenuPaths(db, "/plugins/kubernetes/create")
}

const bundledExampleManifest = `id: example
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

func seedBundledExamplePlugin(db *gorm.DB) error {
	now := time.Now().UTC()
	row := pluginModel{
		ID:                 "example",
		Version:            "0.1.0",
		DisplayName:        "Example Plugin",
		ManifestYAML:       bundledExampleManifest,
		InstalledFilesJSON: "[]",
		Status:             string(model.PluginStatusDeployed),
		CreatedBy:          "00000000-0000-0000-0000-000000000000",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	return db.Where("id = ?", "example").FirstOrCreate(&row).Error
}

func seedBundledKubernetesPlugin(db *gorm.DB) error {
	var receipt uninstallReceiptModel
	err := db.Where("plugin_id = ?", "kubernetes").First(&receipt).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var existing pluginModel
	err = db.Select("id").Where("id = ?", "kubernetes").First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	descriptor, ok := sparkplugin.Bundled("kubernetes")
	if !ok {
		return nil
	}
	parsed, err := manifest.Parse([]byte(descriptor.ManifestYAML))
	if err != nil || parsed.ID != "kubernetes" {
		if err != nil {
			return err
		}
		return domain.ErrPluginIDConflict
	}
	installedFiles, err := json.Marshal(descriptor.InstalledFiles)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	row := pluginModel{
		ID:                 "kubernetes",
		Version:            parsed.Version,
		DisplayName:        parsed.DisplayName,
		ManifestYAML:       descriptor.ManifestYAML,
		InstalledFilesJSON: string(installedFiles),
		Status:             string(model.PluginStatusDeployed),
		CreatedBy:          "00000000-0000-0000-0000-000000000000",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	return db.Where("id = ?", "kubernetes").FirstOrCreate(&row).Error
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
	db := s.db.WithContext(ctx)
	var existing pluginModel
	err = db.Where("id = ?", plugin.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		createErr := db.Create(&row).Error
		if isUniqueViolation(createErr) {
			return domain.ErrPluginIDConflict
		}
		return createErr
	}
	if err != nil {
		return err
	}
	if existing.PackageDigest != "" && row.PackageDigest != "" && existing.PackageDigest != row.PackageDigest {
		return domain.ErrPluginIDConflict
	}
	if conflictingInstalledFiles(existing.InstalledFilesJSON, row.InstalledFilesJSON) {
		return domain.ErrPluginIDConflict
	}
	result := db.Model(&pluginModel{}).
		Where("id = ? AND package_digest = ?", existing.ID, existing.PackageDigest).
		Updates(map[string]any{
			"version":         row.Version,
			"display_name":    row.DisplayName,
			"manifest_yaml":   row.ManifestYAML,
			"package_digest":  row.PackageDigest,
			"installed_files": row.InstalledFilesJSON,
			"status":          row.Status,
			"pr_url":          row.PRURL,
			"updated_at":      row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrPluginIDConflict
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}

func conflictingInstalledFiles(existingJSON, incomingJSON string) bool {
	if existingJSON == "" || existingJSON == "[]" || incomingJSON == "" {
		return false
	}
	var existing, incoming []string
	if json.Unmarshal([]byte(existingJSON), &existing) != nil {
		return false
	}
	if json.Unmarshal([]byte(incomingJSON), &incoming) != nil {
		return false
	}
	if len(existing) == 0 {
		return false
	}
	return !slices.Equal(existing, incoming)
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

func pruneRemovedPluginMenuPaths(db *gorm.DB, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	if !db.Migrator().HasTable("governance.menus") && !db.Migrator().HasTable("menus") {
		return nil
	}
	now := time.Now().UTC()
	result := db.Exec(
		"UPDATE governance.menus SET deleted_at = ?, updated_at = ? WHERE path IN ? AND deleted_at IS NULL",
		now, now, paths,
	)
	if result.Error != nil && strings.Contains(result.Error.Error(), "no such table") {
		return nil
	}
	return result.Error
}

var _ repository.PluginRepository = (*Store)(nil)
