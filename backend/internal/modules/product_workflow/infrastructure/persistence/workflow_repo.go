package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type productWorkflowModel struct {
	ID              string `gorm:"column:id;primaryKey"`
	ProductID       string
	WorkspaceID     string
	Name            string
	Key             string `gorm:"column:key"`
	Description     string `gorm:"type:text"`
	Status          string
	Stages          string `gorm:"type:jsonb"`
	TemplateBinding string `gorm:"type:jsonb"`
	Origin          string `gorm:"type:jsonb"`
	CreatedBy       string
	postgres.AuditColumns
	DeletedAt gorm.DeletedAt
}

func (productWorkflowModel) TableName() string { return "product.product_workflows" }

type productWorkflowRunModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	WorkflowID   string
	ProductID    string
	Status       string
	Trigger      string `gorm:"type:jsonb"`
	Input        string `gorm:"type:jsonb"`
	StageRuns    string `gorm:"type:jsonb"`
	CurrentStage int
	ErrorMessage string `gorm:"type:text"`
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
}

func (productWorkflowRunModel) TableName() string { return "product.product_workflow_runs" }

type workflowBuildRefModel struct {
	BuildID   string `gorm:"column:build_id;primaryKey"`
	RunID     string
	JobName   string
	ProjectID string
	CreatedAt time.Time
}

func (workflowBuildRefModel) TableName() string { return "product.workflow_build_refs" }

type workflowDeployRefModel struct {
	DeploymentRunID string `gorm:"column:deployment_run_id;primaryKey"`
	RunID           string
	JobName         string
	ProjectID       string
	CreatedAt       time.Time
}

func (workflowDeployRefModel) TableName() string { return "product.workflow_deploy_refs" }

type ProductWorkflowRepository struct{ db *gorm.DB }

func NewProductWorkflowRepository(db *gorm.DB) repository.ProductWorkflowRepository {
	return &ProductWorkflowRepository{db: db}
}

func workflowAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&productWorkflowModel{}, &productWorkflowRunModel{}, &workflowBuildRefModel{}, &workflowDeployRefModel{})
}

func (r *ProductWorkflowRepository) SaveWorkflow(ctx context.Context, wf *model.ProductWorkflow) error {
	row, err := toWorkflowModel(wf)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ProductWorkflowRepository) UpdateWorkflow(ctx context.Context, wf *model.ProductWorkflow) error {
	stages, err := json.Marshal(wf.Stages)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&productWorkflowModel{}).Where("id = ?", wf.ID.String()).Updates(map[string]any{
		"name": wf.Name, "description": wf.Description, "status": wf.Status,
		"stages": string(stages), "updated_at": time.Now().UTC(),
	}).Error
}

func (r *ProductWorkflowRepository) FindWorkflowByID(ctx context.Context, id kernel.ProductWorkflowID) (*model.ProductWorkflow, error) {
	var row productWorkflowModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProductWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToWorkflow(row)
}

func (r *ProductWorkflowRepository) ListWorkflowsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.ProductWorkflow, error) {
	var rows []productWorkflowModel
	err := r.db.WithContext(ctx).Where("workspace_id = ?", wsID.String()).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProductWorkflow, 0, len(rows))
	for _, row := range rows {
		wf, err := rowToWorkflow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, wf)
	}
	return out, nil
}

func (r *ProductWorkflowRepository) ListWorkflowsByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ProductWorkflow, error) {
	var rows []productWorkflowModel
	err := r.db.WithContext(ctx).Where("product_id = ?", productID.String()).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProductWorkflow, 0, len(rows))
	for _, row := range rows {
		wf, err := rowToWorkflow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, wf)
	}
	return out, nil
}

func (r *ProductWorkflowRepository) SaveRun(ctx context.Context, run *model.ProductWorkflowRun) error {
	row, err := toRunModel(run)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ProductWorkflowRepository) UpdateRun(ctx context.Context, run *model.ProductWorkflowRun) error {
	row, err := toRunModel(run)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&productWorkflowRunModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status": row.Status, "stage_runs": row.StageRuns, "current_stage": row.CurrentStage,
		"error_message": row.ErrorMessage, "started_at": row.StartedAt, "completed_at": row.CompletedAt,
	}).Error
}

func (r *ProductWorkflowRepository) FindRunByID(ctx context.Context, id kernel.ProductWorkflowRunID) (*model.ProductWorkflowRun, error) {
	var row productWorkflowRunModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProductWorkflowRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToRun(row)
}

func (r *ProductWorkflowRepository) ListRunsByWorkflow(ctx context.Context, workflowID kernel.ProductWorkflowID, limit int) ([]*model.ProductWorkflowRun, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []productWorkflowRunModel
	err := r.db.WithContext(ctx).Where("workflow_id = ?", workflowID.String()).Order("created_at DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProductWorkflowRun, 0, len(rows))
	for _, row := range rows {
		run, err := rowToRun(row)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func (r *ProductWorkflowRepository) SaveBuildRef(ctx context.Context, ref *model.WorkflowBuildRef) error {
	return r.db.WithContext(ctx).Create(&workflowBuildRefModel{
		BuildID: ref.BuildID.String(), RunID: ref.RunID.String(),
		JobName: ref.JobName, ProjectID: ref.ProjectID.String(), CreatedAt: time.Now().UTC(),
	}).Error
}

func (r *ProductWorkflowRepository) FindRunByBuildID(ctx context.Context, buildID kernel.BuildID) (*model.WorkflowBuildRef, error) {
	var row workflowBuildRefModel
	err := r.db.WithContext(ctx).Where("build_id = ?", buildID.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProductWorkflowRunNotFound
	}
	if err != nil {
		return nil, err
	}
	runID, err := kernel.ParseProductWorkflowRunID(row.RunID)
	if err != nil {
		return nil, err
	}
	projectID, err := kernel.ParseProjectID(row.ProjectID)
	if err != nil {
		return nil, err
	}
	return &model.WorkflowBuildRef{
		BuildID: buildID, RunID: runID, JobName: row.JobName, ProjectID: projectID,
	}, nil
}

func (r *ProductWorkflowRepository) SaveDeployRef(ctx context.Context, ref *model.WorkflowDeployRef) error {
	return r.db.WithContext(ctx).Create(&workflowDeployRefModel{
		DeploymentRunID: ref.DeploymentRunID.String(), RunID: ref.RunID.String(),
		JobName: ref.JobName, ProjectID: ref.ProjectID.String(), CreatedAt: time.Now().UTC(),
	}).Error
}

func (r *ProductWorkflowRepository) FindRunByDeploymentID(ctx context.Context, deploymentID kernel.DeploymentRunID) (*model.WorkflowDeployRef, error) {
	var row workflowDeployRefModel
	err := r.db.WithContext(ctx).Where("deployment_run_id = ?", deploymentID.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProductWorkflowRunNotFound
	}
	if err != nil {
		return nil, err
	}
	runID, err := kernel.ParseProductWorkflowRunID(row.RunID)
	if err != nil {
		return nil, err
	}
	projectID, err := kernel.ParseProjectID(row.ProjectID)
	if err != nil {
		return nil, err
	}
	return &model.WorkflowDeployRef{
		DeploymentRunID: deploymentID, RunID: runID, JobName: row.JobName, ProjectID: projectID,
	}, nil
}

func toWorkflowModel(wf *model.ProductWorkflow) (productWorkflowModel, error) {
	stages, err := json.Marshal(wf.Stages)
	if err != nil {
		return productWorkflowModel{}, err
	}
	tb := string(wf.TemplateBinding)
	if tb == "" {
		tb = "null"
	}
	origin := string(wf.Origin)
	if origin == "" {
		origin = `{}`
	}
	return productWorkflowModel{
		ID: wf.ID.String(), ProductID: wf.ProductID.String(), WorkspaceID: wf.WorkspaceID.String(),
		Name: wf.Name, Key: wf.Key, Description: wf.Description, Status: string(wf.Status),
		Stages: string(stages), TemplateBinding: tb, Origin: origin, CreatedBy: wf.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: wf.CreatedAt, UpdatedAt: wf.UpdatedAt},
	}, nil
}

func rowToWorkflow(row productWorkflowModel) (*model.ProductWorkflow, error) {
	id, err := kernel.ParseProductWorkflowID(row.ID)
	if err != nil {
		return nil, err
	}
	productID, err := kernel.ParseProductID(row.ProductID)
	if err != nil {
		return nil, err
	}
	wsID, err := kernel.ParseWorkspaceID(row.WorkspaceID)
	if err != nil {
		return nil, err
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, err
	}
	var stages []model.WorkflowStage
	if row.Stages != "" {
		var wrapper struct {
			Stages []model.WorkflowStage `json:"stages"`
		}
		if err := json.Unmarshal([]byte(row.Stages), &wrapper); err == nil && len(wrapper.Stages) > 0 {
			stages = wrapper.Stages
		} else {
			_ = json.Unmarshal([]byte(row.Stages), &stages)
		}
	}
	return &model.ProductWorkflow{
		ID: id, ProductID: productID, WorkspaceID: wsID, Name: row.Name, Key: row.Key,
		Description: row.Description, Status: model.WorkflowStatus(row.Status), Stages: stages,
		TemplateBinding: json.RawMessage(row.TemplateBinding), Origin: json.RawMessage(row.Origin),
		CreatedBy: createdBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func toRunModel(run *model.ProductWorkflowRun) (productWorkflowRunModel, error) {
	stageRuns, err := json.Marshal(run.StageRuns)
	if err != nil {
		return productWorkflowRunModel{}, err
	}
	trigger := string(run.Trigger)
	if trigger == "" {
		trigger = `{}`
	}
	input := string(run.Input)
	if input == "" {
		input = `{}`
	}
	return productWorkflowRunModel{
		ID: run.ID.String(), WorkflowID: run.WorkflowID.String(), ProductID: run.ProductID.String(),
		Status: string(run.Status), Trigger: trigger, Input: input, StageRuns: string(stageRuns),
		CurrentStage: run.CurrentStage, ErrorMessage: run.ErrorMessage,
		StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, CreatedAt: run.CreatedAt,
	}, nil
}

func rowToRun(row productWorkflowRunModel) (*model.ProductWorkflowRun, error) {
	id, err := kernel.ParseProductWorkflowRunID(row.ID)
	if err != nil {
		return nil, err
	}
	workflowID, err := kernel.ParseProductWorkflowID(row.WorkflowID)
	if err != nil {
		return nil, err
	}
	productID, err := kernel.ParseProductID(row.ProductID)
	if err != nil {
		return nil, err
	}
	var stageRuns []model.StageRun
	if row.StageRuns != "" {
		_ = json.Unmarshal([]byte(row.StageRuns), &stageRuns)
	}
	return &model.ProductWorkflowRun{
		ID: id, WorkflowID: workflowID, ProductID: productID, Status: model.RunStatus(row.Status),
		Trigger: json.RawMessage(row.Trigger), Input: json.RawMessage(row.Input),
		StageRuns: stageRuns, CurrentStage: row.CurrentStage, ErrorMessage: row.ErrorMessage,
		StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt,
	}, nil
}
