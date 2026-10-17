package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type releasePlanFlowDeps struct {
	plans      repository.ReleasePlanRepository
	executor   *service.ReleasePlanExecutor
	approval   *service.ReleasePlanApprovalNotifier
	hooks      *service.ReleasePlanHookNotifier
	operations messaging.OperationWriter
}

func (d releasePlanFlowDeps) persistJobs(ctx context.Context, plan *model.ReleasePlan) error {
	for i := range plan.Jobs {
		if err := d.plans.UpdateJob(ctx, &plan.Jobs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (d releasePlanFlowDeps) sendHook(ctx context.Context, plan *model.ReleasePlan, event model.ReleasePlanHookEvent) error {
	if err := service.ValidateReleasePlanHook(plan, event); err != nil {
		return err
	}
	if d.operations != nil {
		ctx = releasePlanOperationContext(ctx, plan, "hook:"+string(event), hookToken(plan))
		return service.ScheduleReleasePlanWebhook(ctx, d.operations, productoutbound.ReleasePlanWebhookPayload{
			PlanID: plan.ID.String(), Event: string(event), Token: hookToken(plan),
		})
	}
	if d.hooks == nil {
		return nil
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return d.hooks.Send(sendCtx, plan, event)
}

func (d releasePlanFlowDeps) beginHookWait(plan *model.ReleasePlan, event model.ReleasePlanHookEvent) {
	now := time.Now().UTC()
	plan.InitHookWait(event, uuid.NewString(), now)
}

func (d releasePlanFlowDeps) afterPlanningComplete(ctx context.Context, plan *model.ReleasePlan, actor kernel.MemberID) error {
	switch plan.Status {
	case model.ReleasePlanStatusWaitForApproval:
		if err := service.ValidateReleasePlanApproval(plan); err != nil {
			return err
		}
		initApprovalToken(plan)
		if err := d.plans.UpdatePlan(ctx, plan); err != nil {
			return err
		}
		if d.operations != nil || d.approval != nil {
			if err := d.sendApproval(ctx, plan); err != nil {
				return err
			}
		}
	case model.ReleasePlanStatusWaitExecuteExternalCheck:
		d.beginHookWait(plan, model.ReleasePlanHookEventBeforeExecute)
		if err := d.plans.UpdatePlan(ctx, plan); err != nil {
			return err
		}
		if err := d.sendHook(ctx, plan, model.ReleasePlanHookEventBeforeExecute); err != nil {
			return err
		}
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

func (d releasePlanFlowDeps) sendApproval(ctx context.Context, plan *model.ReleasePlan) error {
	if err := service.ValidateReleasePlanApproval(plan); err != nil {
		return err
	}
	if d.operations != nil {
		ctx = releasePlanOperationContext(ctx, plan, "approval", approvalToken(plan))
		return service.ScheduleReleasePlanApproval(ctx, d.operations, productoutbound.ReleasePlanApprovalPayload{
			PlanID: plan.ID.String(),
		})
	}
	if d.approval == nil {
		return nil
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return d.approval.NotifyApproval(sendCtx, plan)
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
	if err := service.ValidateReleasePlanHook(plan, model.ReleasePlanHookEventAllDone); err != nil {
		return err
	}
	d.beginHookWait(plan, model.ReleasePlanHookEventAllDone)
	if err := d.plans.UpdatePlan(ctx, plan); err != nil {
		return err
	}
	_ = auditReleasePlan(ctx, d.plans, plan.ID, actor, "wait_all_done_hook", nil)
	return d.sendHook(ctx, plan, model.ReleasePlanHookEventAllDone)
}

func validatePlanningExternalConfig(plan *model.ReleasePlan) error {
	if err := service.ValidateReleasePlanHook(plan, model.ReleasePlanHookEventFinishPlanning); err != nil {
		return err
	}
	if err := service.ValidateReleasePlanHook(plan, model.ReleasePlanHookEventBeforeExecute); err != nil {
		return err
	}
	return service.ValidateReleasePlanApproval(plan)
}

func validateApprovalExternalConfig(plan *model.ReleasePlan) error {
	if err := service.ValidateReleasePlanApproval(plan); err != nil {
		return err
	}
	return service.ValidateReleasePlanHook(plan, model.ReleasePlanHookEventBeforeExecute)
}

func releasePlanOperationContext(ctx context.Context, plan *model.ReleasePlan, action, token string) context.Context {
	if _, ok := events.MetadataFromContext(ctx); ok {
		return ctx
	}
	seed := "product.release-plan:" + plan.ID.String() + ":" + action + ":" + token
	return events.WithMetadata(ctx, events.Metadata{
		EventID:    uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed)).String(),
		Subscriber: "product.release-plan",
	})
}

func hookToken(plan *model.ReleasePlan) string {
	if plan == nil || plan.HookState == nil {
		return "-"
	}
	return plan.HookState.Token
}

func approvalToken(plan *model.ReleasePlan) string {
	if plan == nil || plan.ApprovalConfig.Instance == nil {
		return "-"
	}
	return plan.ApprovalConfig.Instance.Token
}

func withinReleasePlanOperationTx[T any](
	ctx context.Context,
	writer messaging.OperationWriter,
	tx postgres.Transactor,
	fn func(context.Context) (T, error),
) (T, error) {
	if writer == nil {
		return fn(ctx)
	}
	var zero T
	if tx == nil {
		return zero, fmt.Errorf("release plan operation transaction is required")
	}
	var result T
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		result, err = fn(txCtx)
		return err
	})
	if err != nil {
		return zero, err
	}
	return result, nil
}
