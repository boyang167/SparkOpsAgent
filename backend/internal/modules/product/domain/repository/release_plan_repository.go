package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanRepository interface {
	SavePlan(ctx context.Context, plan *model.ReleasePlan) error
	UpdatePlan(ctx context.Context, plan *model.ReleasePlan) error
	FindPlanByID(ctx context.Context, id kernel.ReleasePlanID) (*model.ReleasePlan, error)
	ListPlansByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ReleasePlan, error)
	SaveJob(ctx context.Context, job *model.ReleasePlanJob) error
	UpdateJob(ctx context.Context, job *model.ReleasePlanJob) error
	DeleteJob(ctx context.Context, jobID kernel.ReleasePlanJobID) error
	ListJobsByPlan(ctx context.Context, planID kernel.ReleasePlanID) ([]model.ReleasePlanJob, error)
	SaveAuditLog(ctx context.Context, log *model.ReleasePlanAuditLog) error
	ListAuditLogs(ctx context.Context, planID kernel.ReleasePlanID, limit int) ([]model.ReleasePlanAuditLog, error)
	FindJobByWorkflowRunID(ctx context.Context, runID kernel.ProductWorkflowRunID) (*model.ReleasePlanJob, *model.ReleasePlan, error)
}
