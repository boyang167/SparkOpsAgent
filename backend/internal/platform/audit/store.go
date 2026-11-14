package audit

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type GormStore struct {
	db *gorm.DB
}

func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

func (s *GormStore) AutoMigrate() error {
	if !postgres.ShouldRunAutoMigrate(s.db) {
		return nil
	}
	if s.db.Dialector.Name() == "postgres" {
		if err := s.db.Exec("CREATE SCHEMA IF NOT EXISTS platform").Error; err != nil {
			return err
		}
	}
	return s.db.AutoMigrate(&auditLogModel{})
}

type auditLogModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	ActorID      string `gorm:"column:actor_id;not null"`
	Action       string `gorm:"column:action;not null"`
	ResourceType string `gorm:"column:resource_type;not null"`
	ResourceID   string `gorm:"column:resource_id;not null"`
	OrgID        *string
	Detail       json.RawMessage `gorm:"column:detail;type:jsonb"`
	CreatedAt    time.Time
}

func (auditLogModel) TableName() string { return "platform.audit_logs" }

func (s *GormStore) Append(ctx context.Context, entry Entry) error {
	orgID := (*string)(nil)
	if entry.OrgID != nil {
		v := entry.OrgID.String()
		orgID = &v
	}
	row := auditLogModel{
		ID:           uuid.New().String(),
		ActorID:      entry.ActorID.String(),
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		OrgID:        orgID,
		Detail:       EncodeDetail(entry.Detail),
		CreatedAt:    time.Now().UTC(),
	}
	return postgres.DB(ctx, s.db).Create(&row).Error
}

func (s *GormStore) List(ctx context.Context, filter ListFilter) ([]LogView, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := postgres.DB(ctx, s.db).Model(&auditLogModel{}).Order("created_at DESC, id DESC").Limit(limit).Offset(filter.Offset)
	if filter.OrgID != nil {
		q = q.Where("org_id = ?", filter.OrgID.String())
	}
	if prefix := strings.TrimSpace(filter.ActionPrefix); prefix != "" {
		q = q.Where("action LIKE ?", prefix+"%")
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		q = q.Where("action = ?", action)
	}
	if actor := strings.TrimSpace(filter.ActorID); actor != "" {
		q = q.Where("actor_id = ?", actor)
	}
	if resourceID := strings.TrimSpace(filter.ResourceID); resourceID != "" {
		q = q.Where("resource_id = ?", resourceID)
	}
	if resourceType := strings.TrimSpace(filter.ResourceType); resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	if workspaceID := strings.TrimSpace(filter.WorkspaceID); workspaceID != "" {
		if s.db.Dialector.Name() == "postgres" {
			q = q.Where("detail ->> 'workspace_id' = ?", workspaceID)
		} else {
			q = q.Where("json_extract(detail, '$.workspace_id') = ?", workspaceID)
		}
	}
	if decision := strings.TrimSpace(filter.Decision); decision != "" {
		legacyAllowAction := strings.TrimSpace(filter.AllowMissingDecisionAction)
		if s.db.Dialector.Name() == "postgres" {
			if strings.EqualFold(decision, "ALLOW") && legacyAllowAction != "" {
				q = q.Where(
					"(detail ->> 'decision' = ? OR (action = ? AND detail ->> 'decision' IS NULL))",
					decision, legacyAllowAction,
				)
			} else {
				q = q.Where("detail ->> 'decision' = ?", decision)
			}
		} else {
			if strings.EqualFold(decision, "ALLOW") && legacyAllowAction != "" {
				q = q.Where(
					"(json_extract(detail, '$.decision') = ? OR (action = ? AND json_extract(detail, '$.decision') IS NULL))",
					decision, legacyAllowAction,
				)
			} else {
				q = q.Where("json_extract(detail, '$.decision') = ?", decision)
			}
		}
	}
	if !filter.CreatedAtAfter.IsZero() {
		q = q.Where("created_at >= ?", filter.CreatedAtAfter)
	}
	if !filter.CreatedAtBefore.IsZero() {
		q = q.Where("created_at <= ?", filter.CreatedAtBefore)
	}
	var rows []auditLogModel
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]LogView, 0, len(rows))
	for _, row := range rows {
		view := LogView{
			ID: row.ID, ActorID: row.ActorID, Action: row.Action,
			ResourceType: row.ResourceType, ResourceID: row.ResourceID,
			Detail: DecodeDetail(row.Detail), CreatedAt: FormatTime(row.CreatedAt),
		}
		if row.OrgID != nil {
			view.OrgID = *row.OrgID
		}
		out = append(out, view)
	}
	return out, nil
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Log(ctx context.Context, entry Entry) error {
	return s.store.Append(ctx, entry)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]LogView, error) {
	return s.store.List(ctx, filter)
}

var _ Logger = (*Service)(nil)
