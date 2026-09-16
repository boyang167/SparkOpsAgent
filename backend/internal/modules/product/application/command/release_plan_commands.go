package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateReleasePlanInput struct {
	Name               string
	Description        string
	ReleaseWindowStart time.Time
	ReleaseWindowEnd   time.Time
	ApprovalEnabled    bool
}

type CreateReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewCreateReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *CreateReleasePlanHandler {
	return &CreateReleasePlanHandler{products: products, plans: plans}
}

func (h *CreateReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID, input CreateReleasePlanInput) (*model.ReleasePlan, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	plan := model.NewReleasePlan(productID, input.Name, input.Description, actor, actor, input.ReleaseWindowStart, input.ReleaseWindowEnd, model.ApprovalConfig{Enabled: input.ApprovalEnabled})
	if err := h.plans.SavePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "create", map[string]any{"name": plan.Name})
	return plan, nil
}

type AddReleasePlanJobInput struct {
	Name        string
	Type        model.ReleasePlanJobType
	TextContent string
	SortOrder   int
	WorkflowRef json.RawMessage
}

type AddReleasePlanJobHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewAddReleasePlanJobHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *AddReleasePlanJobHandler {
	return &AddReleasePlanJobHandler{products: products, plans: plans}
}

func (h *AddReleasePlanJobHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID, input AddReleasePlanJobInput) (*model.ReleasePlanJob, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if plan.Status != model.ReleasePlanStatusPlanning {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	now := time.Now().UTC()
	if input.Type == model.ReleasePlanJobTypeWorkflow && len(input.WorkflowRef) == 0 {
		return nil, domain.ErrInvalidWorkflowRef
	}
	job := &model.ReleasePlanJob{
		ID: kernel.NewReleasePlanJobID(), ReleasePlanID: plan.ID, SortOrder: input.SortOrder,
		Name: input.Name, Type: input.Type, OwnerID: actor, Status: model.ReleasePlanJobStatusTodo,
		TextContent: input.TextContent, WorkflowRef: input.WorkflowRef, CreatedAt: now, UpdatedAt: now,
	}
	if err := h.plans.SaveJob(ctx, job); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "add_job", map[string]any{"job_id": job.ID.String(), "name": job.Name})
	return job, nil
}

type FinishReleasePlanPlanningHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
}

func NewFinishReleasePlanPlanningHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, executor *service.ReleasePlanExecutor) *FinishReleasePlanPlanningHandler {
	return &FinishReleasePlanPlanningHandler{products: products, plans: plans, executor: executor}
}

func (h *FinishReleasePlanPlanningHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.FinishPlanning(); err != nil {
		return nil, err
	}
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "finish_planning", map[string]any{"status": plan.Status})
	if plan.Status == model.ReleasePlanStatusExecuting && h.executor != nil {
		_ = h.executor.Advance(ctx, plan, actor)
	}
	return h.plans.FindPlanByID(ctx, planID)
}

type ApproveReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
}

func NewApproveReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, executor *service.ReleasePlanExecutor) *ApproveReleasePlanHandler {
	return &ApproveReleasePlanHandler{products: products, plans: plans, executor: executor}
}

func (h *ApproveReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.Approve(); err != nil {
		return nil, err
	}
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "approve", nil)
	if h.executor != nil {
		plan, _ = h.plans.FindPlanByID(ctx, planID)
		_ = h.executor.Advance(ctx, plan, actor)
	}
	return h.plans.FindPlanByID(ctx, planID)
}

type RejectReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewRejectReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *RejectReleasePlanHandler {
	return &RejectReleasePlanHandler{products: products, plans: plans}
}

func (h *RejectReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID, reason string) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.Reject(); err != nil {
		return nil, err
	}
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "reject", map[string]any{"reason": reason})
	return plan, nil
}

type MarkReleasePlanJobDoneHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
}

func NewMarkReleasePlanJobDoneHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, executor *service.ReleasePlanExecutor) *MarkReleasePlanJobDoneHandler {
	return &MarkReleasePlanJobDoneHandler{products: products, plans: plans, executor: executor}
}

func (h *MarkReleasePlanJobDoneHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, jobID kernel.ReleasePlanJobID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.MarkTextJobDone(jobID); err != nil {
		return nil, err
	}
	if err := persistPlanJobs(ctx, h.plans, h.executor, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "job_done", map[string]any{"job_id": jobID.String()})
	if plan.Status == model.ReleasePlanStatusExecuting && h.executor != nil {
		plan, _ = h.plans.FindPlanByID(ctx, planID)
		_ = h.executor.Advance(ctx, plan, actor)
	}
	return h.plans.FindPlanByID(ctx, planID)
}

type SkipReleasePlanJobHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
}

func NewSkipReleasePlanJobHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, executor *service.ReleasePlanExecutor) *SkipReleasePlanJobHandler {
	return &SkipReleasePlanJobHandler{products: products, plans: plans, executor: executor}
}

func (h *SkipReleasePlanJobHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, jobID kernel.ReleasePlanJobID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.SkipJob(jobID); err != nil {
		return nil, err
	}
	if err := persistPlanJobs(ctx, h.plans, h.executor, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "job_skip", map[string]any{"job_id": jobID.String()})
	if plan.Status == model.ReleasePlanStatusExecuting && h.executor != nil {
		plan, _ = h.plans.FindPlanByID(ctx, planID)
		_ = h.executor.Advance(ctx, plan, actor)
	}
	return h.plans.FindPlanByID(ctx, planID)
}

type ExecuteReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
}

func NewExecuteReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, executor *service.ReleasePlanExecutor) *ExecuteReleasePlanHandler {
	return &ExecuteReleasePlanHandler{products: products, plans: plans, executor: executor}
}

func (h *ExecuteReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if plan.Status != model.ReleasePlanStatusExecuting {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	if err := plan.EnsureExecutingWindow(time.Now().UTC()); err != nil {
		return nil, err
	}
	if h.executor == nil {
		return plan, nil
	}
	if err := h.executor.Advance(ctx, plan, actor); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "execute", nil)
	return h.plans.FindPlanByID(ctx, planID)
}

type RetryReleasePlanJobHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
}

func NewRetryReleasePlanJobHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, executor *service.ReleasePlanExecutor) *RetryReleasePlanJobHandler {
	return &RetryReleasePlanJobHandler{products: products, plans: plans, executor: executor}
}

func (h *RetryReleasePlanJobHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, jobID kernel.ReleasePlanJobID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.RetryWorkflowJob(jobID); err != nil {
		return nil, err
	}
	if err := persistPlanJobs(ctx, h.plans, h.executor, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "job_retry", map[string]any{"job_id": jobID.String()})
	if h.executor != nil {
		plan, _ = h.plans.FindPlanByID(ctx, planID)
		if err := h.executor.Advance(ctx, plan, actor); err != nil {
			return nil, err
		}
	}
	return h.plans.FindPlanByID(ctx, planID)
}

func persistPlanJobs(ctx context.Context, plans repository.ReleasePlanRepository, executor *service.ReleasePlanExecutor, plan *model.ReleasePlan) error {
	if executor != nil {
		return executor.PersistPlanJobs(ctx, plan)
	}
	for i := range plan.Jobs {
		if err := plans.UpdateJob(ctx, &plan.Jobs[i]); err != nil {
			return err
		}
	}
	return plans.UpdatePlan(ctx, plan)
}

func loadReleasePlan(ctx context.Context, products repository.ProductRepository, plans repository.ReleasePlanRepository, productID kernel.ProductID, planID kernel.ReleasePlanID) (*model.ReleasePlan, error) {
	if _, err := products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	plan, err := plans.FindPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan.ProductID != productID {
		return nil, domain.ErrReleasePlanNotFound
	}
	return plan, nil
}

func (h *CreateReleasePlanHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func (h *AddReleasePlanJobHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func (h *FinishReleasePlanPlanningHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func (h *ApproveReleasePlanHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func (h *RejectReleasePlanHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func (h *MarkReleasePlanJobDoneHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func (h *SkipReleasePlanJobHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func (h *ExecuteReleasePlanHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func (h *RetryReleasePlanJobHandler) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	return auditReleasePlan(ctx, h.plans, planID, actor, action, detail)
}

func auditReleasePlan(ctx context.Context, plans repository.ReleasePlanRepository, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	raw, _ := json.Marshal(detail)
	return plans.SaveAuditLog(ctx, &model.ReleasePlanAuditLog{
		ReleasePlanID: planID, ActorID: actor, Action: action,
		Detail: raw, CreatedAt: time.Now().UTC(),
	})
}
