package command

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanHookCallbackInput struct {
	Pass    bool
	Message string
}

type ReleasePlanHookCallbackHandler struct {
	plans      repository.ReleasePlanRepository
	executor   *service.ReleasePlanExecutor
	approval   *service.ReleasePlanApprovalNotifier
	hooks      *service.ReleasePlanHookNotifier
	operations messaging.OperationWriter
	tx         postgres.Transactor
}

func NewReleasePlanHookCallbackHandler(
	plans repository.ReleasePlanRepository,
	executor *service.ReleasePlanExecutor,
	approval *service.ReleasePlanApprovalNotifier,
	hooks *service.ReleasePlanHookNotifier,
) *ReleasePlanHookCallbackHandler {
	return &ReleasePlanHookCallbackHandler{plans: plans, executor: executor, approval: approval, hooks: hooks}
}

func (h *ReleasePlanHookCallbackHandler) UseOperationOutbox(writer messaging.OperationWriter, tx postgres.Transactor) {
	h.operations, h.tx = writer, tx
}

func (h *ReleasePlanHookCallbackHandler) Handle(ctx context.Context, token string, input ReleasePlanHookCallbackInput) (*model.ReleasePlan, error) {
	return withinReleasePlanOperationTx(ctx, h.operations, h.tx, func(txCtx context.Context) (*model.ReleasePlan, error) {
		return h.handle(txCtx, token, input)
	})
}

func (h *ReleasePlanHookCallbackHandler) handle(ctx context.Context, token string, input ReleasePlanHookCallbackInput) (*model.ReleasePlan, error) {
	plan, err := h.plans.FindPlanByHookToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if plan.HookState == nil || plan.HookState.Status != "pending" {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	now := time.Now().UTC()
	flow := releasePlanFlowDeps{
		plans: h.plans, executor: h.executor, approval: h.approval,
		hooks: h.hooks, operations: h.operations,
	}
	if !input.Pass {
		plan.FailHookCheck(strings.TrimSpace(input.Message), now)
		if err := h.plans.UpdatePlan(ctx, plan); err != nil {
			return nil, err
		}
		_ = auditReleasePlan(ctx, h.plans, plan.ID, kernel.MemberID{}, "hook_failed", map[string]any{
			"event": plan.HookState.Event, "message": input.Message,
		})
		return h.plans.FindPlanByID(ctx, plan.ID)
	}
	event := plan.PassHookCheck(now)
	switch event {
	case model.ReleasePlanHookEventFinishPlanning:
		if err := validatePlanningExternalConfig(plan); err != nil {
			return nil, err
		}
		if err := plan.FinishPlanning(); err != nil {
			return nil, err
		}
		plan.ClearHookState()
		_ = auditReleasePlan(ctx, h.plans, plan.ID, kernel.MemberID{}, "hook_passed", map[string]any{"event": event})
		if err := flow.afterPlanningComplete(ctx, plan, kernel.MemberID{}); err != nil {
			return nil, err
		}
	case model.ReleasePlanHookEventBeforeExecute:
		_ = auditReleasePlan(ctx, h.plans, plan.ID, kernel.MemberID{}, "hook_passed", map[string]any{"event": event})
		if err := flow.afterExecuteHookPass(ctx, plan, kernel.MemberID{}); err != nil {
			return nil, err
		}
	case model.ReleasePlanHookEventAllDone:
		_ = auditReleasePlan(ctx, h.plans, plan.ID, kernel.MemberID{}, "hook_passed", map[string]any{"event": event})
		if err := flow.afterAllDoneHookPass(ctx, plan); err != nil {
			return nil, err
		}
	default:
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	return h.plans.FindPlanByID(ctx, plan.ID)
}
