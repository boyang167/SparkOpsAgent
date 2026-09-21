package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CancelWorkflowRunHandler struct {
	workflows repository.ProductWorkflowRepository
	scheduler *engine.Scheduler
	products  port.ProductChecker
}

func NewCancelWorkflowRunHandler(
	workflows repository.ProductWorkflowRepository,
	scheduler *engine.Scheduler,
	products port.ProductChecker,
) *CancelWorkflowRunHandler {
	return &CancelWorkflowRunHandler{workflows: workflows, scheduler: scheduler, products: products}
}

func (h *CancelWorkflowRunHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	runID kernel.ProductWorkflowRunID,
	message string,
) error {
	if err := ensureWorkflowRun(ctx, h.workflows, h.products, wsID, productID, runID); err != nil {
		return err
	}
	return h.scheduler.CancelRun(ctx, runID, strings.TrimSpace(message))
}
