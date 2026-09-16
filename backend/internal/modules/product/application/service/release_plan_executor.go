package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanExecutor struct {
	plans   repository.ReleasePlanRepository
	trigger appctx.WorkflowRunTrigger
}

func NewReleasePlanExecutor(plans repository.ReleasePlanRepository, trigger appctx.WorkflowRunTrigger) *ReleasePlanExecutor {
	return &ReleasePlanExecutor{plans: plans, trigger: trigger}
}

func (e *ReleasePlanExecutor) Advance(ctx context.Context, plan *model.ReleasePlan, actor kernel.MemberID) error {
	if plan == nil {
		return domain.ErrReleasePlanNotFound
	}
	if plan.Status != model.ReleasePlanStatusExecuting {
		return nil
	}
	if err := plan.EnsureExecutingWindow(time.Now().UTC()); err != nil {
		return err
	}
	idx := plan.CurrentJobIndex()
	if idx < 0 {
		return nil
	}
	job := &plan.Jobs[idx]
	switch job.Status {
	case model.ReleasePlanJobStatusRunning, model.ReleasePlanJobStatusFailed:
		return nil
	case model.ReleasePlanJobStatusTodo:
		if job.Type == model.ReleasePlanJobTypeText {
			return nil
		}
	default:
		return nil
	}
	if job.Type != model.ReleasePlanJobTypeWorkflow {
		return nil
	}
	if e.trigger == nil {
		return domain.ErrInvalidWorkflowRef
	}
	spec, err := plan.ParseWorkflowRef(job)
	if err != nil {
		return err
	}
	workflowID, err := kernel.ParseProductWorkflowID(spec.WorkflowID)
	if err != nil {
		return domain.ErrInvalidWorkflowRef
	}
	runID, err := e.trigger.StartForProduct(ctx, plan.ProductID, workflowID)
	if err != nil {
		return err
	}
	if err := plan.MarkWorkflowJobRunning(job.ID, runID); err != nil {
		return err
	}
	if err := e.plans.UpdateJob(ctx, job); err != nil {
		return err
	}
	if err := e.plans.UpdatePlan(ctx, plan); err != nil {
		return err
	}
	return e.audit(ctx, plan.ID, actor, "job_started", map[string]any{
		"job_id": job.ID.String(), "workflow_run_id": runID.String(),
	})
}

func (e *ReleasePlanExecutor) OnWorkflowRunCompleted(
	ctx context.Context,
	runID kernel.ProductWorkflowRunID,
	success bool,
	errMsg string,
	actor kernel.MemberID,
) error {
	job, plan, err := e.plans.FindJobByWorkflowRunID(ctx, runID)
	if err != nil {
		if err == domain.ErrReleasePlanJobNotFound {
			return nil
		}
		return err
	}
	if plan.Status != model.ReleasePlanStatusExecuting {
		return nil
	}
	if err := plan.CompleteWorkflowJob(job.ID, success, errMsg); err != nil {
		return err
	}
	if err := e.plans.UpdateJob(ctx, job); err != nil {
		return err
	}
	if err := e.plans.UpdatePlan(ctx, plan); err != nil {
		return err
	}
	action := "job_done"
	if !success {
		action = "job_failed"
	}
	if err := e.audit(ctx, plan.ID, actor, action, map[string]any{
		"job_id": job.ID.String(), "workflow_run_id": runID.String(), "error_message": errMsg,
	}); err != nil {
		return err
	}
	if success && plan.Status == model.ReleasePlanStatusExecuting {
		return e.Advance(ctx, plan, actor)
	}
	return nil
}

func (e *ReleasePlanExecutor) audit(ctx context.Context, planID kernel.ReleasePlanID, actor kernel.MemberID, action string, detail map[string]any) error {
	raw, _ := json.Marshal(detail)
	return e.plans.SaveAuditLog(ctx, &model.ReleasePlanAuditLog{
		ReleasePlanID: planID, ActorID: actor, Action: action,
		Detail: raw, CreatedAt: time.Now().UTC(),
	})
}

func (e *ReleasePlanExecutor) PersistPlanJobs(ctx context.Context, plan *model.ReleasePlan) error {
	for i := range plan.Jobs {
		if err := e.plans.UpdateJob(ctx, &plan.Jobs[i]); err != nil {
			return err
		}
	}
	return e.plans.UpdatePlan(ctx, plan)
}
