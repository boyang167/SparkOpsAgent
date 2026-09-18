package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ApproveWorkflowJobHandler struct {
	workflows repository.ProductWorkflowRepository
	scheduler *engine.Scheduler
	products  port.ProductChecker
}

func NewApproveWorkflowJobHandler(
	workflows repository.ProductWorkflowRepository,
	scheduler *engine.Scheduler,
	products port.ProductChecker,
) *ApproveWorkflowJobHandler {
	return &ApproveWorkflowJobHandler{workflows: workflows, scheduler: scheduler, products: products}
}

func (h *ApproveWorkflowJobHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	runID kernel.ProductWorkflowRunID,
	jobName string,
	actor kernel.MemberID,
) error {
	if err := ensureWorkflowRun(ctx, h.workflows, h.products, wsID, productID, runID); err != nil {
		return err
	}
	return h.scheduler.ApproveJob(ctx, runID, jobName, actor)
}

type RejectWorkflowJobHandler struct {
	workflows repository.ProductWorkflowRepository
	scheduler *engine.Scheduler
	products  port.ProductChecker
}

func NewRejectWorkflowJobHandler(
	workflows repository.ProductWorkflowRepository,
	scheduler *engine.Scheduler,
	products port.ProductChecker,
) *RejectWorkflowJobHandler {
	return &RejectWorkflowJobHandler{workflows: workflows, scheduler: scheduler, products: products}
}

func (h *RejectWorkflowJobHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	runID kernel.ProductWorkflowRunID,
	jobName string,
	actor kernel.MemberID,
	message string,
) error {
	if err := ensureWorkflowRun(ctx, h.workflows, h.products, wsID, productID, runID); err != nil {
		return err
	}
	return h.scheduler.RejectJob(ctx, runID, jobName, actor, message)
}

type ContinueManualStageHandler struct {
	workflows repository.ProductWorkflowRepository
	scheduler *engine.Scheduler
	products  port.ProductChecker
}

func NewContinueManualStageHandler(
	workflows repository.ProductWorkflowRepository,
	scheduler *engine.Scheduler,
	products port.ProductChecker,
) *ContinueManualStageHandler {
	return &ContinueManualStageHandler{workflows: workflows, scheduler: scheduler, products: products}
}

func (h *ContinueManualStageHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	runID kernel.ProductWorkflowRunID,
) error {
	if err := ensureWorkflowRun(ctx, h.workflows, h.products, wsID, productID, runID); err != nil {
		return err
	}
	return h.scheduler.ContinueManualStage(ctx, runID)
}

func ensureWorkflowRun(
	ctx context.Context,
	workflows repository.ProductWorkflowRepository,
	products port.ProductChecker,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	runID kernel.ProductWorkflowRunID,
) error {
	productWs, err := products.ProductWorkspace(ctx, productID)
	if err != nil {
		return err
	}
	if productWs != wsID {
		return domain.ErrProductNotInWorkspace
	}
	run, err := workflows.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.ProductID != productID {
		return domain.ErrProductWorkflowRunNotFound
	}
	return nil
}
