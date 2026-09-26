package repository

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanListFilter struct {
	Statuses []model.ReleasePlanStatus
	From     *time.Time
	To       *time.Time
}

type ReleasePlanRepository interface {
	SavePlan(ctx context.Context, plan *model.ReleasePlan) error
	UpdatePlan(ctx context.Context, plan *model.ReleasePlan) error
	FindPlanByID(ctx context.Context, id kernel.ReleasePlanID) (*model.ReleasePlan, error)
	ListPlansByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ReleasePlan, error)
	ListPlansByProductFiltered(ctx context.Context, productID kernel.ProductID, sprintID *kernel.SprintID, filter ReleasePlanListFilter) ([]*model.ReleasePlan, error)
	ListPlansByProductAndSprint(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID) ([]*model.ReleasePlan, error)
	ListPlansByWorkspace(ctx context.Context, wsID kernel.WorkspaceID, filter ReleasePlanListFilter) ([]*model.ReleasePlan, error)
	HasBlockingPlansBySprint(ctx context.Context, sprintID kernel.SprintID) (bool, error)
	ListBlockingPlansBySprint(ctx context.Context, sprintID kernel.SprintID) ([]*model.ReleasePlan, error)
	SaveJob(ctx context.Context, job *model.ReleasePlanJob) error
	UpdateJob(ctx context.Context, job *model.ReleasePlanJob) error
	DeleteJob(ctx context.Context, jobID kernel.ReleasePlanJobID) error
	ListJobsByPlan(ctx context.Context, planID kernel.ReleasePlanID) ([]model.ReleasePlanJob, error)
	SaveAuditLog(ctx context.Context, log *model.ReleasePlanAuditLog) error
	ListAuditLogs(ctx context.Context, planID kernel.ReleasePlanID, limit int) ([]model.ReleasePlanAuditLog, error)
	FindJobByWorkflowRunID(ctx context.Context, runID kernel.ProductWorkflowRunID) (*model.ReleasePlanJob, *model.ReleasePlan, error)
	FindPlanByApprovalToken(ctx context.Context, token string) (*model.ReleasePlan, error)
	ListDueScheduledPlans(ctx context.Context, now time.Time) ([]*model.ReleasePlan, error)
}

type ReleasePlanTemplateRepository interface {
	SaveTemplate(ctx context.Context, tpl *model.ReleasePlanTemplate) error
	UpdateTemplate(ctx context.Context, tpl *model.ReleasePlanTemplate) error
	DeleteTemplate(ctx context.Context, id kernel.ReleasePlanTemplateID) error
	FindTemplateByID(ctx context.Context, id kernel.ReleasePlanTemplateID) (*model.ReleasePlanTemplate, error)
	ListTemplatesByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.ReleasePlanTemplate, error)
}
