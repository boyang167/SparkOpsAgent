package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DeliveryStatusReader struct{ db *gorm.DB }

func NewDeliveryStatusReader(db *gorm.DB) *DeliveryStatusReader {
	return &DeliveryStatusReader{db: db}
}

func (r *DeliveryStatusReader) BuildStatus(ctx context.Context, buildID kernel.BuildID) (status string, terminal bool, err error) {
	var row struct {
		Status string
	}
	err = r.db.WithContext(ctx).Table("delivery.builds").Select("status").Where("id = ?", buildID.String()).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	switch row.Status {
	case "success", "failed", "aborted":
		return row.Status, true, nil
	default:
		return row.Status, false, nil
	}
}

func (r *DeliveryStatusReader) DeployStatus(ctx context.Context, deploymentID kernel.DeploymentRunID) (status string, terminal bool, err error) {
	var row struct {
		Status string
	}
	err = r.db.WithContext(ctx).Table("delivery.deployment_runs").Select("status").Where("id = ?", deploymentID.String()).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	switch row.Status {
	case "succeeded", "failed", "aborted":
		return row.Status, true, nil
	default:
		return row.Status, false, nil
	}
}
