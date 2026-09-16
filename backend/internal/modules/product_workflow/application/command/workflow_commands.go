package command

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/keyval"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateWorkflowFromTemplateInput struct {
	TemplateID kernel.WorkflowTemplateID
	Name       string
	KeyVals    map[string]string
	ProjectIDs []kernel.ProjectID
}

type CreateWorkflowFromTemplateHandler struct {
	templates repository.WorkflowTemplateRepository
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewCreateWorkflowFromTemplateHandler(
	templates repository.WorkflowTemplateRepository,
	workflows repository.ProductWorkflowRepository,
	products port.ProductChecker,
) *CreateWorkflowFromTemplateHandler {
	return &CreateWorkflowFromTemplateHandler{templates: templates, workflows: workflows, products: products}
}

func (h *CreateWorkflowFromTemplateHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	actor kernel.MemberID,
	input CreateWorkflowFromTemplateInput,
) (*model.ProductWorkflow, error) {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return nil, err
	}
	if productWs != wsID {
		return nil, domain.ErrProductNotInWorkspace
	}
	if len(input.ProjectIDs) == 0 {
		return nil, domain.ErrInvalidWorkflowInput
	}
	tpl, err := h.templates.FindByID(ctx, input.TemplateID)
	if err != nil {
		return nil, err
	}
	merged := keyval.MergeKeyVals(tpl.KeyVals, input.KeyVals)
	resolved, err := keyval.Resolve(tpl.Definition, merged)
	if err != nil {
		return nil, err
	}
	stages, err := engine.ParseStagesFromDefinition(resolved)
	if err != nil {
		return nil, err
	}
	defaultBranch := "main"
	for _, kv := range merged {
		if kv.Key == "build_branch" && kv.Value != "" {
			defaultBranch = kv.Value
			break
		}
	}
	engine.InjectProjectBuilds(stages, input.ProjectIDs, defaultBranch)

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = tpl.Name
	}
	key := slugKey(name)
	binding, _ := json.Marshal(map[string]any{
		"template_id": tpl.ID.String(), "template_key": tpl.Key, "key_vals": merged,
	})
	wf := model.NewProductWorkflow(productID, wsID, name, key, stages, actor)
	wf.TemplateBinding = binding
	wf.Description = tpl.Description
	if err := h.workflows.SaveWorkflow(ctx, wf); err != nil {
		return nil, err
	}
	return wf, nil
}

type StartWorkflowRunHandler struct {
	workflows repository.ProductWorkflowRepository
	scheduler *engine.Scheduler
	products  port.ProductChecker
}

func NewStartWorkflowRunHandler(
	workflows repository.ProductWorkflowRepository,
	scheduler *engine.Scheduler,
	products port.ProductChecker,
) *StartWorkflowRunHandler {
	return &StartWorkflowRunHandler{workflows: workflows, scheduler: scheduler, products: products}
}

func (h *StartWorkflowRunHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	workflowID kernel.ProductWorkflowID,
) (*model.ProductWorkflowRun, error) {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return nil, err
	}
	if productWs != wsID {
		return nil, domain.ErrProductNotInWorkspace
	}
	wf, err := h.workflows.FindWorkflowByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if wf.ProductID != productID {
		return nil, domain.ErrProductWorkflowNotFound
	}
	run := model.NewProductWorkflowRun(workflowID, productID, wf.Stages)
	run.Trigger = json.RawMessage(`{"source":"manual"}`)
	if err := h.workflows.SaveRun(ctx, run); err != nil {
		return nil, err
	}
	if err := h.scheduler.StartRun(ctx, wf, run); err != nil {
		return nil, err
	}
	return h.workflows.FindRunByID(ctx, run.ID)
}

func slugKey(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	key = strings.ReplaceAll(key, " ", "-")
	if key == "" {
		key = "workflow-" + time.Now().UTC().Format("20060102150405")
	}
	return key
}
