package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS delivery").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&buildModel{}, &cronjobModel{})
}

type buildModel struct {
	ID                 string `gorm:"column:id;primaryKey"`
	ProjectID          string
	MemberID           string
	WorkspaceID        string
	Status             string
	Branch             string
	BuildNumber        int
	JenkinsJob         string
	ConsoleLog         string `gorm:"type:text"`
	Error              string
	RollbackOfBuildID  string
	StartedAt          time.Time
	ExecutionStartedAt *time.Time
	CompletedAt        *time.Time
	postgres.AuditColumns
}

func (buildModel) TableName() string { return "delivery.builds" }

type BuildRepository struct{ db *gorm.DB }

func NewBuildRepository(db *gorm.DB) repository.BuildRepository {
	return &BuildRepository{db: db}
}

func (r *BuildRepository) Save(ctx context.Context, build *model.Build) error {
	row := buildToRow(build)
	row.AuditColumns = postgres.AuditColumns{CreatedAt: build.StartedAt, UpdatedAt: build.StartedAt}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *BuildRepository) Update(ctx context.Context, build *model.Build) error {
	return r.db.WithContext(ctx).Model(&buildModel{}).Where("id = ?", build.ID.String()).Updates(map[string]any{
		"status":               string(build.Status),
		"build_number":         build.BuildNumber,
		"jenkins_job":          build.JenkinsJob,
		"console_log":          build.ConsoleLog,
		"error":                build.Error,
		"rollback_of_build_id": build.RollbackOfBuildID,
		"completed_at":         build.CompletedAt,
		"updated_at":           time.Now().UTC(),
		"execution_started_at": build.ExecutionStartedAt,
	}).Error
}

func (r *BuildRepository) FindByID(ctx context.Context, id kernel.BuildID) (*model.Build, error) {
	var row buildModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrBuildNotFound
	}
	if err != nil {
		return nil, err
	}
	return buildRowToModel(row)
}

func (r *BuildRepository) ListByProject(ctx context.Context, projectID kernel.ProjectID, limit int) ([]*model.Build, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []buildModel
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID.String()).
		Order("started_at desc").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Build, 0, len(rows))
	for _, row := range rows {
		build, err := buildRowToModel(row)
		if err != nil {
			return nil, err
		}
		out = append(out, build)
	}
	return out, nil
}

func (r *BuildRepository) HasRunningBuild(ctx context.Context, projectID kernel.ProjectID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&buildModel{}).
		Where("project_id = ? AND status = ?", projectID.String(), string(model.BuildStatusRunning)).
		Count(&count).Error
	return count > 0, err
}

func (r *BuildRepository) NextBuildNumber(ctx context.Context, projectID kernel.ProjectID) (int, error) {
	var maxNumber *int
	err := r.db.WithContext(ctx).Model(&buildModel{}).
		Where("project_id = ?", projectID.String()).
		Select("MAX(build_number)").
		Scan(&maxNumber).Error
	if err != nil {
		return 0, err
	}
	if maxNumber == nil {
		return 1, nil
	}
	return *maxNumber + 1, nil
}

func buildToRow(build *model.Build) buildModel {
	return buildModel{
		ID: build.ID.String(), ProjectID: build.ProjectID.String(),
		MemberID: build.MemberID.String(), WorkspaceID: build.WorkspaceID.String(),
		Status: string(build.Status), Branch: build.Branch,
		BuildNumber: build.BuildNumber, JenkinsJob: build.JenkinsJob,
		ConsoleLog: build.ConsoleLog, Error: build.Error,
		RollbackOfBuildID: build.RollbackOfBuildID,
		StartedAt:         build.StartedAt, CompletedAt: build.CompletedAt,
		ExecutionStartedAt: build.ExecutionStartedAt,
	}
}

func buildRowToModel(row buildModel) (*model.Build, error) {
	id, _ := kernel.ParseBuildID(row.ID)
	projectID, _ := kernel.ParseProjectID(row.ProjectID)
	memberID, _ := kernel.ParseMemberID(row.MemberID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	return &model.Build{
		ID: id, ProjectID: projectID, MemberID: memberID, WorkspaceID: wsID,
		Status: model.BuildStatus(row.Status), Branch: row.Branch,
		BuildNumber: row.BuildNumber, JenkinsJob: row.JenkinsJob,
		ConsoleLog: row.ConsoleLog, Error: row.Error,
		RollbackOfBuildID: row.RollbackOfBuildID,
		StartedAt:         row.StartedAt, CompletedAt: row.CompletedAt,
		ExecutionStartedAt: row.ExecutionStartedAt,
	}, nil
}

var _ repository.BuildRepository = (*BuildRepository)(nil)
