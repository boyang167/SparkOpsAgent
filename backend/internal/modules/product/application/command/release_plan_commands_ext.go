package command

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CopyReleasePlanInput struct {
	Name string
}

type CopyReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewCopyReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *CopyReleasePlanHandler {
	return &CopyReleasePlanHandler{products: products, plans: plans}
}

func (h *CopyReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, actor kernel.MemberID, input CopyReleasePlanInput) (*model.ReleasePlan, error) {
	src, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = src.Name + " (副本)"
	}
	now := time.Now().UTC()
	cfg := src.ApprovalConfig
	cfg.Instance = nil
	clone := model.NewReleasePlan(productID, name, src.Description, src.ManagerID, actor, src.ReleaseWindowStart, src.ReleaseWindowEnd, cfg)
	clone.ScheduleExecuteTime = src.ScheduleExecuteTime
	clone.Parameters = maps.Clone(src.Parameters)
	if src.ApproverID != nil {
		approver := *src.ApproverID
		clone.ApproverID = &approver
	}
	if err := h.plans.SavePlan(ctx, clone); err != nil {
		return nil, err
	}
	snap := model.SnapshotFromPlan(src)
	for _, job := range snap.Jobs {
		newJob := &model.ReleasePlanJob{
			ID: kernel.NewReleasePlanJobID(), ReleasePlanID: clone.ID, SortOrder: job.SortOrder,
			Name: job.Name, Type: job.Type, OwnerID: actor, Status: model.ReleasePlanJobStatusTodo,
			TextContent: job.TextContent, TextFormat: job.TextFormat, WorkflowRef: job.WorkflowRef,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := h.plans.SaveJob(ctx, newJob); err != nil {
			return nil, err
		}
	}
	_ = auditReleasePlan(ctx, h.plans, clone.ID, actor, "copy", map[string]any{"source_plan_id": planID.String()})
	clonePlan, err := h.plans.FindPlanByID(ctx, clone.ID)
	if err != nil {
		return nil, err
	}
	_ = persistReleasePlanVersion(ctx, h.plans, clonePlan, actor, "copy")
	return clonePlan, nil
}

type SaveReleasePlanTemplateInput struct {
	Name        string
	Description string
}

type SaveReleasePlanTemplateHandler struct {
	products  repository.ProductRepository
	plans     repository.ReleasePlanRepository
	templates repository.ReleasePlanTemplateRepository
}

func NewSaveReleasePlanTemplateHandler(
	products repository.ProductRepository,
	plans repository.ReleasePlanRepository,
	templates repository.ReleasePlanTemplateRepository,
) *SaveReleasePlanTemplateHandler {
	return &SaveReleasePlanTemplateHandler{products: products, plans: plans, templates: templates}
}

func (h *SaveReleasePlanTemplateHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	planID kernel.ReleasePlanID,
	actor kernel.MemberID,
	input SaveReleasePlanTemplateInput,
) (*model.ReleasePlanTemplate, error) {
	product, err := h.products.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.WorkspaceID != wsID {
		return nil, domain.ErrProductNotFound
	}
	plan, err := loadReleasePlan(ctx, h.products, h.plans, productID, planID)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = plan.Name + " 模板"
	}
	tpl := model.NewReleasePlanTemplate(wsID, name, input.Description, model.SnapshotFromPlan(plan), actor)
	if err := h.templates.SaveTemplate(ctx, tpl); err != nil {
		return nil, err
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "save_template", map[string]any{"template_id": tpl.ID.String()})
	return tpl, nil
}

type CreateReleasePlanFromTemplateInput struct {
	ProductID           kernel.ProductID
	Name                string
	Description         string
	ManagerID           kernel.MemberID
	ReleaseWindowStart  time.Time
	ReleaseWindowEnd    time.Time
	ScheduleExecuteTime *time.Time
	SprintID            *kernel.SprintID
}

type CreateReleasePlanFromTemplateHandler struct {
	products  repository.ProductRepository
	plans     repository.ReleasePlanRepository
	templates repository.ReleasePlanTemplateRepository
	sprints   repository.SprintRepository
}

func NewCreateReleasePlanFromTemplateHandler(
	products repository.ProductRepository,
	plans repository.ReleasePlanRepository,
	templates repository.ReleasePlanTemplateRepository,
	sprints repository.SprintRepository,
) *CreateReleasePlanFromTemplateHandler {
	return &CreateReleasePlanFromTemplateHandler{products: products, plans: plans, templates: templates, sprints: sprints}
}

func (h *CreateReleasePlanFromTemplateHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	templateID kernel.ReleasePlanTemplateID,
	actor kernel.MemberID,
	input CreateReleasePlanFromTemplateInput,
) (*model.ReleasePlan, error) {
	tpl, err := h.templates.FindTemplateByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if tpl.WorkspaceID != wsID {
		return nil, domain.ErrReleasePlanTemplateNotFound
	}
	product, err := h.products.FindProductByID(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product.WorkspaceID != wsID {
		return nil, domain.ErrProductNotFound
	}
	if input.SprintID != nil {
		sprint, err := h.sprints.FindSprintByID(ctx, *input.SprintID)
		if err != nil {
			return nil, err
		}
		if sprint.ProductID != input.ProductID {
			return nil, domain.ErrSprintNotFound
		}
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = tpl.Name
	}
	desc := input.Description
	if desc == "" {
		desc = tpl.Snapshot.Description
	}
	managerID := input.ManagerID
	if managerID == (kernel.MemberID{}) {
		managerID = actor
	}
	cfg := tpl.Snapshot.ApprovalConfig
	cfg.Instance = nil
	plan := model.NewReleasePlan(input.ProductID, name, desc, managerID, actor, input.ReleaseWindowStart, input.ReleaseWindowEnd, cfg)
	if input.ScheduleExecuteTime != nil {
		plan.ScheduleExecuteTime = input.ScheduleExecuteTime
	} else {
		plan.ScheduleExecuteTime = tpl.Snapshot.ScheduleExecuteTime
	}
	plan.SprintID = input.SprintID
	plan.Parameters = maps.Clone(tpl.Snapshot.Parameters)
	if err := h.plans.SavePlan(ctx, plan); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, job := range tpl.Snapshot.Jobs {
		newJob := &model.ReleasePlanJob{
			ID: kernel.NewReleasePlanJobID(), ReleasePlanID: plan.ID, SortOrder: job.SortOrder,
			Name: job.Name, Type: job.Type, OwnerID: actor, Status: model.ReleasePlanJobStatusTodo,
			TextContent: job.TextContent, TextFormat: job.TextFormat, WorkflowRef: job.WorkflowRef,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := h.plans.SaveJob(ctx, newJob); err != nil {
			return nil, err
		}
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, actor, "create_from_template", map[string]any{"template_id": templateID.String()})
	created, err := h.plans.FindPlanByID(ctx, plan.ID)
	if err != nil {
		return nil, err
	}
	_ = persistReleasePlanVersion(ctx, h.plans, created, actor, "create_from_template")
	return created, nil
}

type ListReleasePlanTemplatesHandler struct {
	templates repository.ReleasePlanTemplateRepository
}

func NewListReleasePlanTemplatesHandler(templates repository.ReleasePlanTemplateRepository) *ListReleasePlanTemplatesHandler {
	return &ListReleasePlanTemplatesHandler{templates: templates}
}

func (h *ListReleasePlanTemplatesHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.ReleasePlanTemplate, error) {
	return h.templates.ListTemplatesByWorkspace(ctx, wsID)
}

type DeleteReleasePlanTemplateHandler struct {
	templates repository.ReleasePlanTemplateRepository
}

func NewDeleteReleasePlanTemplateHandler(templates repository.ReleasePlanTemplateRepository) *DeleteReleasePlanTemplateHandler {
	return &DeleteReleasePlanTemplateHandler{templates: templates}
}

func (h *DeleteReleasePlanTemplateHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, templateID kernel.ReleasePlanTemplateID) error {
	tpl, err := h.templates.FindTemplateByID(ctx, templateID)
	if err != nil {
		return err
	}
	if tpl.WorkspaceID != wsID {
		return domain.ErrReleasePlanTemplateNotFound
	}
	return h.templates.DeleteTemplate(ctx, templateID)
}

type ApprovalByTokenInput struct {
	Note string
}

type ApproveReleasePlanByTokenHandler struct {
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
	hooks    *service.ReleasePlanHookNotifier
}

func NewApproveReleasePlanByTokenHandler(
	plans repository.ReleasePlanRepository,
	executor *service.ReleasePlanExecutor,
	hooks *service.ReleasePlanHookNotifier,
) *ApproveReleasePlanByTokenHandler {
	return &ApproveReleasePlanByTokenHandler{plans: plans, executor: executor, hooks: hooks}
}

func (h *ApproveReleasePlanByTokenHandler) Handle(ctx context.Context, token string, input ApprovalByTokenInput) (*model.ReleasePlan, error) {
	plan, err := h.plans.FindPlanByApprovalToken(ctx, token)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := plan.ApproveByToken(strings.TrimSpace(input.Note), now); err != nil {
		return nil, err
	}
	flow := releasePlanFlowDeps{plans: h.plans, executor: h.executor, hooks: h.hooks}
	if plan.Status == model.ReleasePlanStatusWaitExecuteExternalCheck {
		flow.beginHookWait(plan, model.ReleasePlanHookEventBeforeExecute)
		if err := h.plans.UpdatePlan(ctx, plan); err != nil {
			return nil, err
		}
		_ = auditReleasePlan(ctx, h.plans, plan.ID, kernel.MemberID{}, "approve_by_token", map[string]any{"note": input.Note, "hook": "before_execute"})
		flow.sendHook(ctx, plan, model.ReleasePlanHookEventBeforeExecute)
		return h.plans.FindPlanByID(ctx, plan.ID)
	}
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, kernel.MemberID{}, "approve_by_token", map[string]any{"note": input.Note})
	maybeAdvanceReleasePlan(ctx, h.plans, h.executor, plan.ID, kernel.MemberID{})
	return h.plans.FindPlanByID(ctx, plan.ID)
}

type RejectReleasePlanByTokenHandler struct {
	plans repository.ReleasePlanRepository
}

func NewRejectReleasePlanByTokenHandler(plans repository.ReleasePlanRepository) *RejectReleasePlanByTokenHandler {
	return &RejectReleasePlanByTokenHandler{plans: plans}
}

func (h *RejectReleasePlanByTokenHandler) Handle(ctx context.Context, token string, input ApprovalByTokenInput) (*model.ReleasePlan, error) {
	plan, err := h.plans.FindPlanByApprovalToken(ctx, token)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := plan.RejectByToken(strings.TrimSpace(input.Note), now); err != nil {
		return nil, err
	}
	if err := h.plans.UpdatePlan(ctx, plan); err != nil {
		return nil, err
	}
	_ = auditReleasePlan(ctx, h.plans, plan.ID, kernel.MemberID{}, "reject_by_token", map[string]any{"note": input.Note})
	return plan, nil
}

func maybeAdvanceReleasePlan(
	ctx context.Context,
	plans repository.ReleasePlanRepository,
	executor *service.ReleasePlanExecutor,
	planID kernel.ReleasePlanID,
	actor kernel.MemberID,
) {
	if executor == nil {
		return
	}
	plan, err := plans.FindPlanByID(ctx, planID)
	if err != nil || !service.ShouldAdvancePlan(plan, time.Now().UTC()) {
		return
	}
	_ = executor.Advance(ctx, plan, actor)
}

func initApprovalToken(plan *model.ReleasePlan) {
	if plan == nil || !plan.ApprovalConfig.UsesExternalApproval() {
		return
	}
	plan.InitApprovalInstance(uuid.NewString(), time.Now().UTC())
}

func cloneApprovalConfig(input *model.ApprovalConfig) model.ApprovalConfig {
	if input == nil {
		return model.ApprovalConfig{}
	}
	cfg := *input
	cfg.Instance = nil
	return cfg
}

func mergeApprovalConfig(current model.ApprovalConfig, patch *model.ApprovalConfig) model.ApprovalConfig {
	if patch == nil {
		return current
	}
	out := current
	out.Enabled = patch.Enabled
	if patch.Provider != "" {
		out.Provider = patch.Provider
	}
	if patch.WebhookURL != "" {
		out.WebhookURL = patch.WebhookURL
	}
	if patch.ApproverIDs != nil {
		out.ApproverIDs = patch.ApproverIDs
	}
	return out
}

func FormatApprovalHTML(title, message string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>%s</title></head><body><h2>%s</h2><p>%s</p></body></html>`, title, title, message)
}
