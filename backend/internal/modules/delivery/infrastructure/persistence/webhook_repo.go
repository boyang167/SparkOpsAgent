package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type webhookModel struct {
	ID        string `gorm:"column:id;primaryKey"`
	ProjectID string
	Name      string
	URL       string `gorm:"column:url"`
	Token     string
	Events    string `gorm:"type:jsonb"`
	Enabled   bool
	postgres.AuditColumns
}

func (webhookModel) TableName() string { return "delivery.project_webhooks" }

type WebhookRecord struct {
	ID        kernel.WebhookID
	ProjectID kernel.ProjectID
	Name      string
	URL       string
	Token     string
	Events    []string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WebhookRepository struct{ db *gorm.DB }

func NewWebhookRepository(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

func (r *WebhookRepository) Save(ctx context.Context, hook *WebhookRecord) error {
	now := hook.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	created := hook.CreatedAt
	if created.IsZero() {
		created = now
	}
	row := webhookModel{
		ID: hook.ID.String(), ProjectID: hook.ProjectID.String(),
		Name: hook.Name, URL: hook.URL, Token: hook.Token,
		Events: encodeWebhookEvents(hook.Events), Enabled: hook.Enabled,
		AuditColumns: postgres.AuditColumns{CreatedAt: created, UpdatedAt: now},
	}
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *WebhookRepository) Get(ctx context.Context, id kernel.WebhookID) (*WebhookRecord, error) {
	var row webhookModel
	if err := postgres.DB(ctx, r.db).Where("id = ?", id.String()).First(&row).Error; err != nil {
		return nil, err
	}
	return rowToWebhook(row)
}

func (r *WebhookRepository) ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]WebhookRecord, error) {
	var rows []webhookModel
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID.String()).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToWebhooks(rows)
}

func (r *WebhookRepository) ListEnabledByProject(ctx context.Context, projectID kernel.ProjectID) ([]WebhookRecord, error) {
	var rows []webhookModel
	if err := postgres.DB(ctx, r.db).Where("project_id = ? AND enabled = ?", projectID.String(), true).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToWebhooks(rows)
}

func (r *WebhookRepository) Delete(ctx context.Context, id kernel.WebhookID) error {
	res := r.db.WithContext(ctx).Delete(&webhookModel{}, "id = ?", id.String())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("webhook not found")
	}
	return nil
}

func encodeWebhookEvents(events []string) string {
	if len(events) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(events)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func rowsToWebhooks(rows []webhookModel) ([]WebhookRecord, error) {
	out := make([]WebhookRecord, 0, len(rows))
	for _, row := range rows {
		item, err := rowToWebhook(row)
		if err != nil {
			continue
		}
		out = append(out, *item)
	}
	return out, nil
}

func rowToWebhook(row webhookModel) (*WebhookRecord, error) {
	id, err := kernel.ParseWebhookID(row.ID)
	if err != nil {
		return nil, err
	}
	pid, err := kernel.ParseProjectID(row.ProjectID)
	if err != nil {
		return nil, err
	}
	return &WebhookRecord{
		ID: id, ProjectID: pid, Name: row.Name, URL: row.URL, Token: row.Token,
		Events: decodeWebhookEvents(row.Events), Enabled: row.Enabled,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func decodeWebhookEvents(raw string) []string {
	if raw == "" {
		return nil
	}
	var events []string
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		return nil
	}
	return events
}
