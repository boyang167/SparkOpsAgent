package acl

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintUsageChecker struct{ db *gorm.DB }

func NewSprintUsageChecker(db *gorm.DB) port.SprintUsageChecker {
	return &SprintUsageChecker{db: db}
}

func (a *SprintUsageChecker) CountSprintsByProcess(ctx context.Context, processID kernel.SprintProcessID) (int, error) {
	var count int64
	if err := a.db.WithContext(ctx).Table("product.sprints").
		Where("process_id = ?", processID.String()).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}
