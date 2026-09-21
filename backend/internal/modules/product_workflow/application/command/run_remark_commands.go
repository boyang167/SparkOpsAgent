package command

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UpdateWorkflowRunRemarkHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewUpdateWorkflowRunRemarkHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker) *UpdateWorkflowRunRemarkHandler {
	return &UpdateWorkflowRunRemarkHandler{workflows: workflows, products: products}
}

func (h *UpdateWorkflowRunRemarkHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	runID kernel.ProductWorkflowRunID,
	remark string,
) error {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return err
	}
	if productWs != wsID {
		return domain.ErrProductNotInWorkspace
	}
	run, err := h.workflows.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.ProductID != productID {
		return domain.ErrProductWorkflowRunNotFound
	}
	stored := map[string]any{}
	if len(run.Input) > 0 {
		_ = json.Unmarshal(run.Input, &stored)
	}
	stored["remark"] = strings.TrimSpace(remark)
	raw, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	run.Input = raw
	return h.workflows.UpdateRun(ctx, run)
}
