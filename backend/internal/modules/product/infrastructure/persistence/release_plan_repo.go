package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type releasePlanModel struct {
	ID                  string `gorm:"column:id;primaryKey"`
	ProductID           string
	ProductPlanID       *string
	Name                string
	Description         string `gorm:"type:text"`
	ManagerID           string
	ApproverID          *string
	Status              string
	ReleaseWindowStart  time.Time
	ReleaseWindowEnd    time.Time
	ScheduleExecuteTime *time.Time
	ApprovalConfig      string `gorm:"type:jsonb"`
	CreatedBy           string
	postgres.AuditColumns
	DeletedAt gorm.DeletedAt
}

func (releasePlanModel) TableName() string { return "product.release_plans" }

type releasePlanJobModel struct {
	ID            string `gorm:"column:id;primaryKey"`
	ReleasePlanID string
	SortOrder     int
	Name          string
	Type          string
	OwnerID       string
	Status        string
	TextContent   string `gorm:"type:text"`
	TextChecked   bool
	WorkflowRef   string `gorm:"type:jsonb"`
	ExecutionRef  string `gorm:"type:jsonb"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (releasePlanJobModel) TableName() string { return "product.release_plan_jobs" }

type releasePlanAuditLogModel struct {
	ID            string `gorm:"column:id;primaryKey"`
	ReleasePlanID string
	ActorID       string
	Action        string
	Detail        string `gorm:"type:jsonb"`
	CreatedAt     time.Time
}

func (releasePlanAuditLogModel) TableName() string { return "product.release_plan_audit_logs" }

type ReleasePlanRepository struct{ db *gorm.DB }

func NewReleasePlanRepository(db *gorm.DB) repository.ReleasePlanRepository {
	return &ReleasePlanRepository{db: db}
}

func releasePlanAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&releasePlanModel{}, &releasePlanJobModel{}, &releasePlanAuditLogModel{})
}

func (r *ReleasePlanRepository) SavePlan(ctx context.Context, plan *model.ReleasePlan) error {
	row, err := toReleasePlanModel(plan)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleasePlanRepository) UpdatePlan(ctx context.Context, plan *model.ReleasePlan) error {
	row, err := toReleasePlanModel(plan)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&releasePlanModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"name": row.Name, "description": row.Description, "manager_id": row.ManagerID,
		"approver_id": row.ApproverID, "status": row.Status,
		"release_window_start": row.ReleaseWindowStart, "release_window_end": row.ReleaseWindowEnd,
		"schedule_execute_time": row.ScheduleExecuteTime, "approval_config": row.ApprovalConfig,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *ReleasePlanRepository) FindPlanByID(ctx context.Context, id kernel.ReleasePlanID) (*model.ReleasePlan, error) {
	var row releasePlanModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReleasePlanNotFound
	}
	if err != nil {
		return nil, err
	}
	plan, err := rowToReleasePlan(row)
	if err != nil {
		return nil, err
	}
	jobs, err := r.ListJobsByPlan(ctx, id)
	if err != nil {
		return nil, err
	}
	plan.Jobs = jobs
	return plan, nil
}

func (r *ReleasePlanRepository) ListPlansByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ReleasePlan, error) {
	var rows []releasePlanModel
	err := r.db.WithContext(ctx).Where("product_id = ?", productID.String()).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.ReleasePlan, 0, len(rows))
	for _, row := range rows {
		plan, err := rowToReleasePlan(row)
		if err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func (r *ReleasePlanRepository) SaveJob(ctx context.Context, job *model.ReleasePlanJob) error {
	row, err := toReleasePlanJobModel(job)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleasePlanRepository) UpdateJob(ctx context.Context, job *model.ReleasePlanJob) error {
	row, err := toReleasePlanJobModel(job)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&releasePlanJobModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"sort_order": row.SortOrder, "name": row.Name, "type": row.Type, "owner_id": row.OwnerID,
		"status": row.Status, "text_content": row.TextContent, "text_checked": row.TextChecked,
		"workflow_ref": row.WorkflowRef, "execution_ref": row.ExecutionRef, "updated_at": time.Now().UTC(),
	}).Error
}

func (r *ReleasePlanRepository) DeleteJob(ctx context.Context, jobID kernel.ReleasePlanJobID) error {
	return r.db.WithContext(ctx).Where("id = ?", jobID.String()).Delete(&releasePlanJobModel{}).Error
}

func (r *ReleasePlanRepository) ListJobsByPlan(ctx context.Context, planID kernel.ReleasePlanID) ([]model.ReleasePlanJob, error) {
	var rows []releasePlanJobModel
	err := r.db.WithContext(ctx).Where("release_plan_id = ?", planID.String()).Order("sort_order ASC, created_at ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]model.ReleasePlanJob, 0, len(rows))
	for _, row := range rows {
		job, err := rowToReleasePlanJob(row)
		if err != nil {
			return nil, err
		}
		out = append(out, *job)
	}
	return out, nil
}

func (r *ReleasePlanRepository) SaveAuditLog(ctx context.Context, log *model.ReleasePlanAuditLog) error {
	detail := string(log.Detail)
	if detail == "" {
		detail = "{}"
	}
	row := releasePlanAuditLogModel{
		ID: uuid.NewString(), ReleasePlanID: log.ReleasePlanID.String(),
		ActorID: log.ActorID.String(), Action: log.Action, Detail: detail,
		CreatedAt: log.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleasePlanRepository) FindJobByWorkflowRunID(ctx context.Context, runID kernel.ProductWorkflowRunID) (*model.ReleasePlanJob, *model.ReleasePlan, error) {
	var row releasePlanJobModel
	err := r.db.WithContext(ctx).
		Where("execution_ref->>'workflow_run_id' = ?", runID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, domain.ErrReleasePlanJobNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	job, err := rowToReleasePlanJob(row)
	if err != nil {
		return nil, nil, err
	}
	plan, err := r.FindPlanByID(ctx, job.ReleasePlanID)
	if err != nil {
		return nil, nil, err
	}
	for i := range plan.Jobs {
		if plan.Jobs[i].ID == job.ID {
			return &plan.Jobs[i], plan, nil
		}
	}
	return nil, nil, domain.ErrReleasePlanJobNotFound
}

func (r *ReleasePlanRepository) ListAuditLogs(ctx context.Context, planID kernel.ReleasePlanID, limit int) ([]model.ReleasePlanAuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []releasePlanAuditLogModel
	err := r.db.WithContext(ctx).Where("release_plan_id = ?", planID.String()).Order("created_at DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]model.ReleasePlanAuditLog, 0, len(rows))
	for _, row := range rows {
		actorID, _ := kernel.ParseMemberID(row.ActorID)
		planIDParsed, _ := kernel.ParseReleasePlanID(row.ReleasePlanID)
		out = append(out, model.ReleasePlanAuditLog{
			ID: row.ID, ReleasePlanID: planIDParsed, ActorID: actorID,
			Action: row.Action, Detail: json.RawMessage(row.Detail), CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func toReleasePlanModel(plan *model.ReleasePlan) (releasePlanModel, error) {
	approval, err := json.Marshal(plan.ApprovalConfig)
	if err != nil {
		return releasePlanModel{}, err
	}
	row := releasePlanModel{
		ID: plan.ID.String(), ProductID: plan.ProductID.String(), Name: plan.Name,
		Description: plan.Description, ManagerID: plan.ManagerID.String(), Status: string(plan.Status),
		ReleaseWindowStart: plan.ReleaseWindowStart, ReleaseWindowEnd: plan.ReleaseWindowEnd,
		ScheduleExecuteTime: plan.ScheduleExecuteTime, ApprovalConfig: string(approval),
		CreatedBy: plan.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: plan.CreatedAt, UpdatedAt: plan.UpdatedAt},
	}
	if plan.ProductPlanID != nil {
		v := plan.ProductPlanID.String()
		row.ProductPlanID = &v
	}
	if plan.ApproverID != nil {
		v := plan.ApproverID.String()
		row.ApproverID = &v
	}
	return row, nil
}

func rowToReleasePlan(row releasePlanModel) (*model.ReleasePlan, error) {
	id, err := kernel.ParseReleasePlanID(row.ID)
	if err != nil {
		return nil, err
	}
	productID, err := kernel.ParseProductID(row.ProductID)
	if err != nil {
		return nil, err
	}
	managerID, err := kernel.ParseMemberID(row.ManagerID)
	if err != nil {
		return nil, err
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, err
	}
	var approval model.ApprovalConfig
	_ = json.Unmarshal([]byte(row.ApprovalConfig), &approval)
	plan := &model.ReleasePlan{
		ID: id, ProductID: productID, Name: row.Name, Description: row.Description,
		ManagerID: managerID, Status: model.ReleasePlanStatus(row.Status),
		ReleaseWindowStart: row.ReleaseWindowStart, ReleaseWindowEnd: row.ReleaseWindowEnd,
		ScheduleExecuteTime: row.ScheduleExecuteTime, ApprovalConfig: approval,
		CreatedBy: createdBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.ProductPlanID != nil {
		pp, err := kernel.ParseProductPlanID(*row.ProductPlanID)
		if err == nil {
			plan.ProductPlanID = &pp
		}
	}
	if row.ApproverID != nil {
		approver, err := kernel.ParseMemberID(*row.ApproverID)
		if err == nil {
			plan.ApproverID = &approver
		}
	}
	return plan, nil
}

func toReleasePlanJobModel(job *model.ReleasePlanJob) (releasePlanJobModel, error) {
	wf := string(job.WorkflowRef)
	if wf == "" {
		wf = "null"
	}
	ex := string(job.ExecutionRef)
	if ex == "" {
		ex = "null"
	}
	return releasePlanJobModel{
		ID: job.ID.String(), ReleasePlanID: job.ReleasePlanID.String(), SortOrder: job.SortOrder,
		Name: job.Name, Type: string(job.Type), OwnerID: job.OwnerID.String(), Status: string(job.Status),
		TextContent: job.TextContent, TextChecked: job.TextChecked,
		WorkflowRef: wf, ExecutionRef: ex, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
	}, nil
}

func rowToReleasePlanJob(row releasePlanJobModel) (*model.ReleasePlanJob, error) {
	id, err := kernel.ParseReleasePlanJobID(row.ID)
	if err != nil {
		return nil, err
	}
	planID, err := kernel.ParseReleasePlanID(row.ReleasePlanID)
	if err != nil {
		return nil, err
	}
	ownerID, err := kernel.ParseMemberID(row.OwnerID)
	if err != nil {
		return nil, err
	}
	return &model.ReleasePlanJob{
		ID: id, ReleasePlanID: planID, SortOrder: row.SortOrder, Name: row.Name,
		Type: model.ReleasePlanJobType(row.Type), OwnerID: ownerID, Status: model.ReleasePlanJobStatus(row.Status),
		TextContent: row.TextContent, TextChecked: row.TextChecked,
		WorkflowRef: json.RawMessage(row.WorkflowRef), ExecutionRef: json.RawMessage(row.ExecutionRef),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}
