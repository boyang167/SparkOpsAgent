package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type releasePlanFlowDeps struct {
	plans    repository.ReleasePlanRepository
	executor *service.ReleasePlanExecutor
	approval *service.ReleasePlanApprovalNotifier
	hooks    *service.ReleasePlanHookNotifier
}

func (d releasePlanFlowDeps) persistJobs(ctx context.Context, plan *model.ReleasePlan) error {
	for i := range plan.Jobs {
		if err := d.plans.UpdateJob(ctx, &plan.Jobs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (d releasePlanFlowDeps) sendHook(ctx context.Context, plan *model.ReleasePlan, event model.ReleasePlanHookEvent) {
	if d.hooks == nil {
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = d.hooks.Send(sendCtx, plan, event)
}

func (d releasePlanFlowDeps) beginHookWait(plan *model.ReleasePlan, event model.ReleasePlanHookEvent) {
	now := time.Now().UTC()
	plan.InitHookWait(event, uuid.NewString(), now)
}

func (d releasePlanFlowDeps) afterPlanningComplete(ctx context.Context, plan *model.ReleasePlan, actor kernel.MemberID) error {
	switch plan.Status {
	case model.ReleasePlanStatusWaitForApproval:
		initApprovalToken(plan)
		if err := d.plans.UpdatePlan(ctx, plan); err != nil {
			return err
		}
		if d.approval != nil {
			d.sendApproval(ctx, plan)
		}
	case model.ReleasePlanStatusWaitExecuteExternalCheck:
		d.beginHookWait(plan, model.ReleasePlanHookEventBeforeExecute)
		if err := d.plans.UpdatePlan(ctx, plan); err != nil {
			return err
		}
		d.sendHook(ctx, plan, model.ReleasePlanHookEventBeforeExecute)
	case model.ReleasePlanStatusExecuting:
		if err := d.plans.UpdatePlan(ctx, plan); err != nil {
			return err
		}
		maybeAdvanceReleasePlan(ctx, d.plans, d.executor, plan.ID, actor)
	default:
		if err := d.plans.UpdatePlan(ctx, plan); err != nil {
			return err
		}
	}
	return nil
}

func (d releasePlanFlowDeps) sendApproval(ctx context.Context, plan *model.ReleasePlan) {
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = d.approval.NotifyApproval(sendCtx, plan)
}

func (d releasePlanFlowDeps) afterExecuteHookPass(ctx context.Context, plan *model.ReleasePlan, actor kernel.MemberID) error {
	plan.Status = model.ReleasePlanStatusExecuting
	plan.ClearHookState()
	if err := d.plans.UpdatePlan(ctx, plan); err != nil {
		return err
	}
	maybeAdvanceReleasePlan(ctx, d.plans, d.executor, plan.ID, actor)
	return nil
}

func (d releasePlanFlowDeps) afterAllDoneHookPass(ctx context.Context, plan *model.ReleasePlan) error {
	plan.Status = model.ReleasePlanStatusSuccess
	plan.ClearHookState()
	return d.plans.UpdatePlan(ctx, plan)
}

func (d releasePlanFlowDeps) beginAllDoneHook(ctx context.Context, plan *model.ReleasePlan, actor kernel.MemberID) error {
	d.beginHookWait(plan, model.ReleasePlanHookEventAllDone)
	if err := d.plans.UpdatePlan(ctx, plan); err != nil {
		return err
	}
	_ = auditReleasePlan(ctx, d.plans, plan.ID, actor, "wait_all_done_hook", nil)
	d.sendHook(ctx, plan, model.ReleasePlanHookEventAllDone)
	return nil
}
