package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type cronjobModel struct {
	ID        string `gorm:"column:id;primaryKey"`
	ProjectID string
	Name      string
	Schedule  string
	Branch    string
	Enabled   bool
	postgres.AuditColumns
}

func (cronjobModel) TableName() string { return "delivery.cronjobs" }

type CronJobRecord struct {
	ID        kernel.CronJobID
	ProjectID kernel.ProjectID
	Name      string
	Schedule  string
	Branch    string
	Enabled   bool
	CreatedAt time.Time
}

type CronJobRepository struct{ db *gorm.DB }

func NewCronJobRepository(db *gorm.DB) *CronJobRepository {
	return &CronJobRepository{db: db}
}

func (r *CronJobRepository) Save(ctx context.Context, job *CronJobRecord) error {
	row := cronjobModel{
		ID: job.ID.String(), ProjectID: job.ProjectID.String(),
		Name: job.Name, Schedule: job.Schedule, Branch: job.Branch, Enabled: job.Enabled,
		AuditColumns: postgres.AuditColumns{CreatedAt: job.CreatedAt, UpdatedAt: job.CreatedAt},
	}
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *CronJobRepository) ListEnabled(ctx context.Context) ([]CronJobRecord, error) {
	var rows []cronjobModel
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToCronJobs(rows)
}

func (r *CronJobRepository) ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]CronJobRecord, error) {
	var rows []cronjobModel
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID.String()).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rowsToCronJobs(rows)
}

func rowsToCronJobs(rows []cronjobModel) ([]CronJobRecord, error) {
	out := make([]CronJobRecord, 0, len(rows))
	for _, row := range rows {
		id, err := kernel.ParseCronJobID(row.ID)
		if err != nil {
			continue
		}
		pid, err := kernel.ParseProjectID(row.ProjectID)
		if err != nil {
			continue
		}
		out = append(out, CronJobRecord{
			ID: id, ProjectID: pid, Name: row.Name, Schedule: row.Schedule,
			Branch: row.Branch, Enabled: row.Enabled, CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func (r *CronJobRepository) Delete(ctx context.Context, id kernel.CronJobID) error {
	res := r.db.WithContext(ctx).Delete(&cronjobModel{}, "id = ?", id.String())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("cronjob not found")
	}
	return nil
}
