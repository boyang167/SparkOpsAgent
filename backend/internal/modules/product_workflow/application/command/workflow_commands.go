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

type CreateBlankWorkflowInput struct {
	Name        string
	Key         string
	Description string
	Stages      []model.WorkflowStage
	Activate    bool
}

type CreateBlankWorkflowHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewCreateBlankWorkflowHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker) *CreateBlankWorkflowHandler {
	return &CreateBlankWorkflowHandler{workflows: workflows, products: products}
}

func (h *CreateBlankWorkflowHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	actor kernel.MemberID,
	input CreateBlankWorkflowInput,
) (*model.ProductWorkflow, error) {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return nil, err
	}
	if productWs != wsID {
		return nil, domain.ErrProductNotInWorkspace
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, domain.ErrInvalidWorkflowInput
	}
	key := strings.TrimSpace(input.Key)
	if key == "" {
		key = slugKey(name)
	}
	stages := input.Stages
	if len(stages) == 0 {
		stages = []model.WorkflowStage{{Name: "阶段1", Parallel: false, Jobs: []model.WorkflowJob{}}}
	} else if err := engine.ValidateStages(stages); err != nil {
		return nil, err
	}
	wf := model.NewBlankProductWorkflow(productID, wsID, name, key, strings.TrimSpace(input.Description), stages, actor)
	if input.Activate {
		wf.Status = model.WorkflowStatusActive
	}
	if err := h.workflows.SaveWorkflow(ctx, wf); err != nil {
		return nil, err
	}
	return wf, nil
}

type UpdateWorkflowInput struct {
	Name        *string
	Description *string
	Status      *string
	Stages      []model.WorkflowStage
}

type UpdateWorkflowHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewUpdateWorkflowHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker) *UpdateWorkflowHandler {
	return &UpdateWorkflowHandler{workflows: workflows, products: products}
}

func (h *UpdateWorkflowHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	workflowID kernel.ProductWorkflowID,
	input UpdateWorkflowInput,
) (*model.ProductWorkflow, error) {
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
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, domain.ErrInvalidWorkflowInput
		}
		wf.Name = name
	}
	if input.Description != nil {
		wf.Description = strings.TrimSpace(*input.Description)
	}
	if input.Status != nil {
		status := model.WorkflowStatus(strings.TrimSpace(*input.Status))
		switch status {
		case model.WorkflowStatusDraft, model.WorkflowStatusActive:
			wf.Status = status
		default:
			return nil, domain.ErrInvalidWorkflowInput
		}
	}
	if input.Stages != nil {
		if err := engine.ValidateStages(input.Stages); err != nil {
			return nil, err
		}
		wf.Stages = input.Stages
	}
	wf.UpdatedAt = time.Now().UTC()
	if err := h.workflows.UpdateWorkflow(ctx, wf); err != nil {
		return nil, err
	}
	return wf, nil
}

type DeleteWorkflowHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewDeleteWorkflowHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker) *DeleteWorkflowHandler {
	return &DeleteWorkflowHandler{workflows: workflows, products: products}
}

func (h *DeleteWorkflowHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	workflowID kernel.ProductWorkflowID,
) error {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return err
	}
	if productWs != wsID {
		return domain.ErrProductNotInWorkspace
	}
	wf, err := h.workflows.FindWorkflowByID(ctx, workflowID)
	if err != nil {
		return err
	}
	if wf.ProductID != productID {
		return domain.ErrProductWorkflowNotFound
	}
	return h.workflows.DeleteWorkflow(ctx, workflowID)
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
	input map[string]any,
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
	if wf.Status != model.WorkflowStatusActive {
		return nil, domain.ErrInvalidWorkflowInput
	}
	stages := engine.ApplyRunInput(wf.Stages, input)
	run := model.NewProductWorkflowRun(workflowID, productID, stages)
	run.Trigger = json.RawMessage(`{"source":"manual"}`)
	if input != nil {
		if raw, err := json.Marshal(input); err == nil {
			run.Input = raw
		}
	}
	runWF := *wf
	runWF.Stages = stages
	if err := h.workflows.SaveRun(ctx, run); err != nil {
		return nil, err
	}
	if err := h.scheduler.StartRun(ctx, &runWF, run); err != nil {
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
