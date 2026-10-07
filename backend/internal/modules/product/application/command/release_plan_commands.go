package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateReleasePlanInput struct {
	Name                string
	Description         string
	ManagerID           kernel.MemberID
	ReleaseWindowStart  time.Time
	ReleaseWindowEnd    time.Time
	ApprovalEnabled     bool
	ApprovalConfig      *model.ApprovalConfig
	ScheduleExecuteTime *time.Time
	SprintID            *kernel.SprintID
}

type UpdateReleasePlanInput struct {
	Name                *string
	Description         *string
	ManagerID           *kernel.MemberID
	ApproverID          *kernel.MemberID
	ReleaseWindowStart  *time.Time
	ReleaseWindowEnd    *time.Time
	ApprovalEnabled     *bool
	ApprovalConfig      *model.ApprovalConfig
	HookSettings        *model.ReleasePlanHookSettings
	ScheduleExecuteTime *time.Time
	ClearScheduleTime   bool
	SprintID            *kernel.SprintID
	ClearSprintID       bool
}

type CreateReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	sprints  repository.SprintRepository
}

func NewCreateReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, sprints repository.SprintRepository) *CreateReleasePlanHandler {
	return &CreateReleasePlanHandler{products: products, plans: plans, sprints: sprints}
}

func (h *CreateReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID, input CreateReleasePlanInput) (*model.ReleasePlan, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	if input.SprintID != nil {
		sprint, err := h.sprints.FindSprintByID(ctx, *input.SprintID)
		if err != nil {
			return nil, err
		}
		if sprint.ProductID != productID {
			return nil, domain.ErrSprintNotFound
		}
	}
	managerID := input.ManagerID
	if managerID == (kernel.MemberID{}) {
		managerID = actor
	}
	approval := model.ApprovalConfig{Enabled: input.ApprovalEnabled}
	if input.ApprovalConfig != nil {
		approval = mergeApprovalConfig(approval, input.ApprovalConfig)
	}
	plan := model.NewReleasePlan(productID, input.Name, input.Description, managerID, actor, input.ReleaseWindowStart, input.ReleaseWindowEnd, approval)
	plan.ScheduleExecuteTime = input.ScheduleExecuteTime
	plan.SprintID = input.SprintID
	if err := h.plans.SavePlan(ctx, plan); err != nil {
		return nil, err
	}
	auditDetail := map[string]any{"name": plan.Name}
	if input.SprintID != nil {
		auditDetail["sprint_id"] = input.SprintID.String()
	}
	_ = h.audit(ctx, plan.ID, actor, "create", auditDetail)
	_ = persistReleasePlanVersion(ctx, h.plans, plan, actor, "create")
	return plan, nil
}

type AddReleasePlanJobInput struct {
	Name        string
	Type        model.ReleasePlanJobType
	TextContent string
	TextFormat  string
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
		TextContent: input.TextContent, TextFormat: input.TextFormat, WorkflowRef: input.WorkflowRef, CreatedAt: now, UpdatedAt: now,
	}
	if err := h.plans.SaveJob(ctx, job); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "add_job", map[string]any{"job_id": job.ID.String(), "name": job.Name})
	_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "add_job")
	return job, nil
}

type FinishReleasePlanPlanningHandler struct {
	products   repository.ProductRepository
	plans      repository.ReleasePlanRepository
	executor   *service.ReleasePlanExecutor
	notifier   *service.ReleasePlanApprovalNotifier
	hooks      *service.ReleasePlanHookNotifier
	snapshots  port.WorkflowSnapshotReader
}

func NewFinishReleasePlanPlanningHandler(
	products repository.ProductRepository,
	plans repository.ReleasePlanRepository,
	executor *service.ReleasePlanExecutor,
	notifier *service.ReleasePlanApprovalNotifier,
	hooks *service.ReleasePlanHookNotifier,
	snapshots port.WorkflowSnapshotReader,
) *FinishReleasePlanPlanningHandler {
	return &FinishReleasePlanPlanningHandler{
		products: products, plans: plans, executor: executor,
		notifier: notifier, hooks: hooks, snapshots: snapshots,
	}
}

func (h *FinishReleasePlanPlanningHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if plan.Status != model.ReleasePlanStatusPlanning {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	if err := service.MaterializeWorkflowSnapshots(ctx, plan, h.snapshots); err != nil {
		return nil, err
	}
	flow := releasePlanFlowDeps{plans: h.plans, executor: h.executor, approval: h.notifier, hooks: h.hooks}
	if err := flow.persistJobs(ctx, plan); err != nil {
		return nil, err
	}
	if plan.HookSettings.EventEnabled(model.ReleasePlanHookEventFinishPlanning) {
		flow.beginHookWait(plan, model.ReleasePlanHookEventFinishPlanning)
		if err := h.plans.UpdatePlan(ctx, plan); err != nil {
			return nil, err
		}
		_ = h.audit(ctx, plan.ID, actor, "finish_planning", map[string]any{"status": plan.Status, "hook": "finish_planning"})
		_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "finish_planning")
		flow.sendHookAsync(plan, model.ReleasePlanHookEventFinishPlanning)
		return h.plans.FindPlanByID(ctx, planID)
	}
	if err := plan.FinishPlanning(); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "finish_planning", map[string]any{"status": plan.Status})
	_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "finish_planning")
	if err := flow.afterPlanningComplete(ctx, plan, actor); err != nil {
		return nil, err
	}
	return h.plans.FindPlanByID(ctx, planID)
}

type ApproveReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
	hooks    *service.ReleasePlanHookNotifier
}

func NewApproveReleasePlanHandler(
	products repository.ProductRepository,
	plans repository.ReleasePlanRepository,
	executor *service.ReleasePlanExecutor,
	hooks *service.ReleasePlanHookNotifier,
) *ApproveReleasePlanHandler {
	return &ApproveReleasePlanHandler{products: products, plans: plans, executor: executor, hooks: hooks}
}

func (h *ApproveReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.Approve(); err != nil {
		return nil, err
	}
	flow := releasePlanFlowDeps{plans: h.plans, executor: h.executor, hooks: h.hooks}
	if plan.Status == model.ReleasePlanStatusWaitExecuteExternalCheck {
		flow.beginHookWait(plan, model.ReleasePlanHookEventBeforeExecute)
		if err := h.plans.UpdatePlan(ctx, plan); err != nil {
			return nil, err
		}
		_ = h.audit(ctx, plan.ID, actor, "approve", map[string]any{"hook": "before_execute"})
		flow.sendHookAsync(plan, model.ReleasePlanHookEventBeforeExecute)
		return h.plans.FindPlanByID(ctx, planID)
	}
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "approve", nil)
	maybeAdvanceReleasePlan(ctx, h.plans, h.executor, planID, actor)
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

func (h *MarkReleasePlanJobDoneHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, jobID kernel.ReleasePlanJobID, actor kernel.MemberID, remark string) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	needsHook, err := plan.MarkTextJobDone(jobID, remark)
	if err != nil {
		return nil, err
	}
	if err := persistPlanJobs(ctx, h.plans, h.executor, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "job_done", map[string]any{"job_id": jobID.String(), "remark": remark})
	if needsHook {
		if err := h.executor.BeginAllDoneHook(ctx, plan, actor); err != nil {
			return nil, err
		}
		return h.plans.FindPlanByID(ctx, planID)
	}
	maybeAdvanceReleasePlan(ctx, h.plans, h.executor, planID, actor)
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
	needsHook, err := plan.SkipJob(jobID)
	if err != nil {
		return nil, err
	}
	if err := persistPlanJobs(ctx, h.plans, h.executor, plan); err != nil {
		return nil, err
	}
	_ = h.audit(ctx, plan.ID, actor, "job_skip", map[string]any{"job_id": jobID.String()})
	if needsHook {
		if err := h.executor.BeginAllDoneHook(ctx, plan, actor); err != nil {
			return nil, err
		}
		return h.plans.FindPlanByID(ctx, planID)
	}
	maybeAdvanceReleasePlan(ctx, h.plans, h.executor, planID, actor)
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

type UpdateReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	sprints  repository.SprintRepository
}

func NewUpdateReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, sprints repository.SprintRepository) *UpdateReleasePlanHandler {
	return &UpdateReleasePlanHandler{products: products, plans: plans, sprints: sprints}
}

func (h *UpdateReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID, input UpdateReleasePlanInput) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if plan.Status != model.ReleasePlanStatusPlanning {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	if input.Name != nil {
		plan.Name = *input.Name
	}
	if input.Description != nil {
		plan.Description = *input.Description
	}
	if input.ManagerID != nil {
		plan.ManagerID = *input.ManagerID
	}
	if input.ApproverID != nil {
		plan.ApproverID = input.ApproverID
	}
	if input.ReleaseWindowStart != nil {
		plan.ReleaseWindowStart = *input.ReleaseWindowStart
	}
	if input.ReleaseWindowEnd != nil {
		plan.ReleaseWindowEnd = *input.ReleaseWindowEnd
	}
	if input.ApprovalEnabled != nil {
		plan.ApprovalConfig.Enabled = *input.ApprovalEnabled
	}
	if input.ApprovalConfig != nil {
		plan.ApprovalConfig = mergeApprovalConfig(plan.ApprovalConfig, input.ApprovalConfig)
	}
	if input.HookSettings != nil {
		plan.HookSettings = *input.HookSettings
	}
	if input.ClearScheduleTime {
		plan.ScheduleExecuteTime = nil
	} else if input.ScheduleExecuteTime != nil {
		plan.ScheduleExecuteTime = input.ScheduleExecuteTime
	}
	if input.ClearSprintID {
		plan.SprintID = nil
	} else if input.SprintID != nil {
		sprint, err := h.sprints.FindSprintByID(ctx, *input.SprintID)
		if err != nil {
			return nil, err
		}
		if sprint.ProductID != productID {
			return nil, domain.ErrSprintNotFound
		}
		plan.SprintID = input.SprintID
	}
	plan.UpdatedAt = time.Now().UTC()
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "update", nil)
	_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "update")
	return h.plans.FindPlanByID(ctx, planID)
}

type CancelReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewCancelReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *CancelReleasePlanHandler {
	return &CancelReleasePlanHandler{products: products, plans: plans}
}

func (h *CancelReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.Cancel(); err != nil {
		return nil, err
	}
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "cancel", nil)
	return plan, nil
}

type DeleteReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewDeleteReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *DeleteReleasePlanHandler {
	return &DeleteReleasePlanHandler{products: products, plans: plans}
}

func (h *DeleteReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID) error {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return err
	}
	if err := plan.CanDelete(); err != nil {
		return err
	}
	return h.plans.DeletePlan(ctx, planID)
}

type ReopenReleasePlanPlanningHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewReopenReleasePlanPlanningHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *ReopenReleasePlanPlanningHandler {
	return &ReopenReleasePlanPlanningHandler{products: products, plans: plans}
}

func (h *ReopenReleasePlanPlanningHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if err := plan.ReopenPlanning(); err != nil {
		return nil, err
	}
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "reopen_planning", nil)
	_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "reopen_planning")
	return plan, nil
}

type ReorderReleasePlanJobsInput struct {
	JobIDs []kernel.ReleasePlanJobID
}

type ReorderReleasePlanJobsHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewReorderReleasePlanJobsHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *ReorderReleasePlanJobsHandler {
	return &ReorderReleasePlanJobsHandler{products: products, plans: plans}
}

func (h *ReorderReleasePlanJobsHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID, input ReorderReleasePlanJobsInput) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if plan.Status != model.ReleasePlanStatusPlanning {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	if len(input.JobIDs) != len(plan.Jobs) {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	byID := map[kernel.ReleasePlanJobID]*model.ReleasePlanJob{}
	for i := range plan.Jobs {
		byID[plan.Jobs[i].ID] = &plan.Jobs[i]
	}
	for i, jobID := range input.JobIDs {
		job, ok := byID[jobID]
		if !ok {
			return nil, domain.ErrReleasePlanJobNotFound
		}
		job.SortOrder = i
		job.UpdatedAt = time.Now().UTC()
		if err := h.plans.UpdateJob(ctx, job); err != nil {
			return nil, err
		}
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "reorder_jobs", nil)
	_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "reorder_jobs")
	return h.plans.FindPlanByID(ctx, planID)
}

type UpdateReleasePlanJobInput struct {
	Name        *string
	OwnerID     *kernel.MemberID
	TextContent *string
	TextFormat  *string
	SortOrder   *int
	WorkflowRef json.RawMessage
	HasWorkflow bool
}

type UpdateReleasePlanJobHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewUpdateReleasePlanJobHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *UpdateReleasePlanJobHandler {
	return &UpdateReleasePlanJobHandler{products: products, plans: plans}
}

func (h *UpdateReleasePlanJobHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, jobID kernel.ReleasePlanJobID, actor kernel.MemberID, input UpdateReleasePlanJobInput) (*model.ReleasePlanJob, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if plan.Status != model.ReleasePlanStatusPlanning {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	var job *model.ReleasePlanJob
	for i := range plan.Jobs {
		if plan.Jobs[i].ID == jobID {
			job = &plan.Jobs[i]
			break
		}
	}
	if job == nil {
		return nil, domain.ErrReleasePlanJobNotFound
	}
	if input.Name != nil {
		job.Name = *input.Name
	}
	if input.OwnerID != nil {
		job.OwnerID = *input.OwnerID
	}
	if input.TextContent != nil {
		job.TextContent = *input.TextContent
	}
	if input.TextFormat != nil {
		job.TextFormat = *input.TextFormat
	}
	if input.SortOrder != nil {
		job.SortOrder = *input.SortOrder
	}
	if input.HasWorkflow {
		job.WorkflowRef = input.WorkflowRef
	}
	job.UpdatedAt = time.Now().UTC()
	if err := h.plans.UpdateJob(ctx, job); err != nil {
		return nil, err
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "update_job", map[string]any{"job_id": jobID.String()})
	_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "update_job")
	return job, nil
}

type DeleteReleasePlanJobHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewDeleteReleasePlanJobHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *DeleteReleasePlanJobHandler {
	return &DeleteReleasePlanJobHandler{products: products, plans: plans}
}

func (h *DeleteReleasePlanJobHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, jobID kernel.ReleasePlanJobID, actor kernel.MemberID) error {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return err
	}
	if plan.Status != model.ReleasePlanStatusPlanning {
		return domain.ErrInvalidReleasePlanTransition
	}
	found := false
	for _, job := range plan.Jobs {
		if job.ID == jobID {
			found = true
			break
		}
	}
	if !found {
		return domain.ErrReleasePlanJobNotFound
	}
	if err := h.plans.DeleteJob(ctx, jobID); err != nil {
		return err
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "delete_job", map[string]any{"job_id": jobID.String()})
	_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "delete_job")
	return nil
}

type AddReleasePlanJobsFromWorkItemsInput struct {
	WorkItemIDs []kernel.WorkItemID
}

type AddReleasePlanJobsFromWorkItemsHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	sprints  repository.SprintRepository
}

func NewAddReleasePlanJobsFromWorkItemsHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, sprints repository.SprintRepository) *AddReleasePlanJobsFromWorkItemsHandler {
	return &AddReleasePlanJobsFromWorkItemsHandler{products: products, plans: plans, sprints: sprints}
}

func (h *AddReleasePlanJobsFromWorkItemsHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID, input AddReleasePlanJobsFromWorkItemsInput) (*model.ReleasePlan, error) {
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	if plan.Status != model.ReleasePlanStatusPlanning {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	if plan.SprintID == nil {
		return nil, domain.ErrSprintNotFound
	}
	sortBase := len(plan.Jobs)
	now := time.Now().UTC()
	for i, workItemID := range input.WorkItemIDs {
		item, err := h.sprints.FindWorkItemByID(ctx, workItemID)
		if err != nil {
			return nil, err
		}
		if item.SprintID != *plan.SprintID {
			return nil, domain.ErrWorkItemNotFound
		}
		text := item.Title
		if item.ExternalRef != nil {
			if key, ok := item.ExternalRef["external_key"].(string); ok && key != "" {
				text = key + " — " + item.Title
			}
		}
		job := &model.ReleasePlanJob{
			ID: kernel.NewReleasePlanJobID(), ReleasePlanID: plan.ID, SortOrder: sortBase + i,
			Name: item.Title, Type: model.ReleasePlanJobTypeText, OwnerID: actor,
			Status: model.ReleasePlanJobStatusTodo, TextContent: text, CreatedAt: now, UpdatedAt: now,
		}
		if item.AssigneeID != nil {
			job.OwnerID = *item.AssigneeID
		}
		if err := h.plans.SaveJob(ctx, job); err != nil {
			return nil, err
		}
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "add_jobs_from_work_items", map[string]any{"count": len(input.WorkItemIDs)})
	_ = recordPlanVersionAfterReload(ctx, h.plans, plan.ID, actor, "add_jobs_from_work_items")
	return h.plans.FindPlanByID(ctx, planID)
}
