package persistence

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type dataResourceModel struct {
	ID              string `gorm:"column:id;primaryKey"`
	ResourceKey     string `gorm:"column:resource_key;uniqueIndex"`
	SchemaName      string `gorm:"column:schema_name"`
	PhysicalTable   string `gorm:"column:table_name"`
	OwnerColumn     string `gorm:"column:owner_column"`
	WorkspaceColumn string `gorm:"column:workspace_column"`
	Title           string
	Description     string
	Enabled         bool
	postgres.AuditColumns
}

func (dataResourceModel) TableName() string { return "governance.data_resources" }

type dataFieldModel struct {
	ID             string `gorm:"column:id;primaryKey"`
	ResourceID     string `gorm:"column:resource_id"`
	ColumnName     string `gorm:"column:column_name"`
	Title          string
	SensitiveLevel string `gorm:"column:sensitive_level"`
	MaskStrategy   string `gorm:"column:mask_strategy"`
	CreatedAt      time.Time
}

func (dataFieldModel) TableName() string { return "governance.data_fields" }

type roleDataPolicyModel struct {
	RoleID         string `gorm:"column:role_id;primaryKey"`
	ResourceKey    string `gorm:"column:resource_key;primaryKey"`
	Action         string `gorm:"column:action;primaryKey"`
	DataScope      string `gorm:"column:data_scope"`
	CustomRules    []byte `gorm:"column:custom_rules;type:jsonb"`
	AllowedColumns []byte `gorm:"column:allowed_columns;type:jsonb"`
	DeniedColumns  []byte `gorm:"column:denied_columns;type:jsonb"`
	postgres.AuditColumns
}

func (roleDataPolicyModel) TableName() string { return "governance.role_data_policies" }

type DataResource struct {
	ID              string `json:"id"`
	ResourceKey     string `json:"resource_key"`
	SchemaName      string `json:"schema_name"`
	TableName       string `json:"table_name"`
	OwnerColumn     string `json:"owner_column"`
	WorkspaceColumn string `json:"workspace_column"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Enabled         bool   `json:"enabled"`
}

type RoleDataPolicy struct {
	ResourceKey string          `json:"resource_key"`
	Action      string          `json:"action"`
	DataScope   string          `json:"data_scope"`
	CustomRules json.RawMessage `json:"custom_rules,omitempty"`
}

type DataPolicyStore struct{ db *gorm.DB }

func NewDataPolicyStore(db *gorm.DB) *DataPolicyStore {
	return &DataPolicyStore{db: db}
}

func (s *DataPolicyStore) AutoMigrate() error {
	if s.db.Dialector.Name() == "postgres" {
		if err := s.db.Exec("CREATE SCHEMA IF NOT EXISTS governance").Error; err != nil {
			return err
		}
	}
	if err := s.db.AutoMigrate(&dataResourceModel{}, &dataFieldModel{}, &roleDataPolicyModel{}); err != nil {
		return err
	}
	return s.seedDefaultResources(context.Background())
}

func (s *DataPolicyStore) seedDefaultResources(ctx context.Context) error {
	rows := []dataResourceModel{
		{
			ID: "a1000001-0000-4000-8000-000000000001", ResourceKey: "project",
			SchemaName: "workspace_app", PhysicalTable: "projects",
			OwnerColumn: "created_by", WorkspaceColumn: "workspace_id",
			Title: "项目", Description: "工作区项目数据", Enabled: true,
			AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
		{
			ID: "a1000001-0000-4000-8000-000000000002", ResourceKey: "agent",
			SchemaName: "agent", PhysicalTable: "agents",
			OwnerColumn: "created_by", WorkspaceColumn: "workspace_id",
			Title: "Agent", Description: "工作区 Agent 数据", Enabled: true,
			AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
		{
			ID: "a1000001-0000-4000-8000-000000000003", ResourceKey: "member",
			SchemaName: "governance", PhysicalTable: "role_bindings",
			OwnerColumn: "member_id", WorkspaceColumn: "workspace_id",
			Title: "成员绑定", Description: "工作区成员角色绑定", Enabled: true,
			AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	for _, row := range rows {
		var count int64
		if err := s.db.WithContext(ctx).Model(&dataResourceModel{}).
			Where("resource_key = ?", row.ResourceKey).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *DataPolicyStore) ListResources(ctx context.Context) ([]DataResource, error) {
	var rows []dataResourceModel
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).
		Order("resource_key asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]DataResource, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDataResource(row))
	}
	return out, nil
}

func (s *DataPolicyStore) ListByRole(ctx context.Context, roleID string) ([]RoleDataPolicy, error) {
	var rows []roleDataPolicyModel
	if err := s.db.WithContext(ctx).Where("role_id = ?", roleID).
		Order("resource_key asc, action asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]RoleDataPolicy, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRoleDataPolicy(row))
	}
	return out, nil
}

func (s *DataPolicyStore) ReplaceForRole(ctx context.Context, roleID string, policies []RoleDataPolicy) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&roleDataPolicyModel{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, p := range policies {
			resourceKey := strings.TrimSpace(p.ResourceKey)
			action := strings.ToLower(strings.TrimSpace(p.Action))
			scope := strings.TrimSpace(p.DataScope)
			if resourceKey == "" || action == "" || scope == "" {
				continue
			}
			customRules := p.CustomRules
			if len(customRules) == 0 {
				customRules = json.RawMessage("[]")
			}
			if err := tx.Create(&roleDataPolicyModel{
				RoleID: roleID, ResourceKey: resourceKey, Action: action, DataScope: scope,
				CustomRules:  customRules,
				AuditColumns: postgres.AuditColumns{CreatedAt: now, UpdatedAt: now},
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *DataPolicyStore) DeleteByRole(ctx context.Context, roleID string) error {
	return s.db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&roleDataPolicyModel{}).Error
}

func toDataResource(row dataResourceModel) DataResource {
	return DataResource{
		ID: row.ID, ResourceKey: row.ResourceKey, SchemaName: row.SchemaName,
		TableName: row.PhysicalTable, OwnerColumn: row.OwnerColumn,
		WorkspaceColumn: row.WorkspaceColumn, Title: row.Title,
		Description: row.Description, Enabled: row.Enabled,
	}
}

func toRoleDataPolicy(row roleDataPolicyModel) RoleDataPolicy {
	custom := json.RawMessage("[]")
	if len(row.CustomRules) > 0 {
		custom = row.CustomRules
	}
	return RoleDataPolicy{
		ResourceKey: row.ResourceKey, Action: row.Action, DataScope: row.DataScope,
		CustomRules: custom,
	}
}
