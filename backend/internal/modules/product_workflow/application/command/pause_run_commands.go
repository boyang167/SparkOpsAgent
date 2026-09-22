package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PauseWorkflowRunHandler struct {
	workflows repository.ProductWorkflowRepository
	scheduler *engine.Scheduler
	products  port.ProductChecker
}

func NewPauseWorkflowRunHandler(
	workflows repository.ProductWorkflowRepository,
	scheduler *engine.Scheduler,
	products port.ProductChecker,
) *PauseWorkflowRunHandler {
	return &PauseWorkflowRunHandler{workflows: workflows, scheduler: scheduler, products: products}
}

func (h *PauseWorkflowRunHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	runID kernel.ProductWorkflowRunID,
) error {
	if err := ensureWorkflowRun(ctx, h.workflows, h.products, wsID, productID, runID); err != nil {
		return err
	}
	return h.scheduler.PauseRun(ctx, runID)
}
