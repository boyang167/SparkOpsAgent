package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RetryWorkflowRunHandler struct {
	workflows repository.ProductWorkflowRepository
	scheduler *engine.Scheduler
	products  port.ProductChecker
}

func NewRetryWorkflowRunHandler(
	workflows repository.ProductWorkflowRepository,
	scheduler *engine.Scheduler,
	products port.ProductChecker,
) *RetryWorkflowRunHandler {
	return &RetryWorkflowRunHandler{workflows: workflows, scheduler: scheduler, products: products}
}

func (h *RetryWorkflowRunHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	runID kernel.ProductWorkflowRunID,
) error {
	if err := ensureWorkflowRun(ctx, h.workflows, h.products, wsID, productID, runID); err != nil {
		return err
	}
	run, err := h.workflows.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusFailed {
		return domain.ErrRunNotRetryable
	}
	wf, err := h.workflows.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return err
	}
	if limit := wf.Advanced.Concurrency; limit > 0 {
		count, err := h.workflows.CountActiveRuns(ctx, wf.ID)
		if err != nil {
			return err
		}
		if count >= int64(limit) {
			return domain.ErrWorkflowRunBusy
		}
	}
	return h.scheduler.RetryRun(ctx, runID)
}
