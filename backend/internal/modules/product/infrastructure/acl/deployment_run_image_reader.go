package acl

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type deploymentRunImageRow struct {
	ID              string `gorm:"column:id;primaryKey"`
	ImageRepository string `gorm:"column:image_repository"`
	ImageTag        string `gorm:"column:image_tag"`
}

func (deploymentRunImageRow) TableName() string { return "delivery.deployment_runs" }

type DeploymentRunImageReader struct{ db *gorm.DB }

func NewDeploymentRunImageReader(db *gorm.DB) port.DeploymentRunImageReader {
	return &DeploymentRunImageReader{db: db}
}

func (r *DeploymentRunImageReader) FindByRunIDs(
	ctx context.Context,
	ids []kernel.DeploymentRunID,
) (map[string]port.DeploymentRunImage, error) {
	out := make(map[string]port.DeploymentRunImage, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	raw := make([]string, 0, len(ids))
	for _, id := range ids {
		value := id.String()
		if value == "" || value == "00000000-0000-0000-0000-000000000000" {
			continue
		}
		raw = append(raw, value)
	}
	if len(raw) == 0 {
		return out, nil
	}
	var rows []deploymentRunImageRow
	if err := r.db.WithContext(ctx).Where("id IN ?", raw).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = port.DeploymentRunImage{
			Repository: row.ImageRepository,
			Tag:        row.ImageTag,
		}
	}
	return out, nil
}
