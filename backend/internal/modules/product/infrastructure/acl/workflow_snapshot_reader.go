package acl

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type workflowSnapshotRow struct {
	ID          string `gorm:"column:id;primaryKey"`
	Name        string
	Key         string `gorm:"column:key"`
	Description string `gorm:"type:text"`
	Status      string
	Stages      string `gorm:"type:jsonb"`
	DeletedAt   gorm.DeletedAt
}

func (workflowSnapshotRow) TableName() string { return "product.product_workflows" }

type WorkflowSnapshotReader struct{ db *gorm.DB }

func NewWorkflowSnapshotReader(db *gorm.DB) port.WorkflowSnapshotReader {
	return &WorkflowSnapshotReader{db: db}
}

func (r *WorkflowSnapshotReader) GetWorkflowSnapshot(ctx context.Context, workflowID kernel.ProductWorkflowID) (json.RawMessage, error) {
	var row workflowSnapshotRow
	err := r.db.WithContext(ctx).Where("id = ?", workflowID.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrInvalidWorkflowRef
	}
	if err != nil {
		return nil, err
	}
	var stages any
	if row.Stages != "" {
		_ = json.Unmarshal([]byte(row.Stages), &stages)
	}
	payload := map[string]any{
		"id": row.ID, "name": row.Name, "key": row.Key,
		"description": row.Description, "status": row.Status,
		"stages": stages, "materialized_at": time.Now().UTC().Format(time.RFC3339),
	}
	return json.Marshal(payload)
}
