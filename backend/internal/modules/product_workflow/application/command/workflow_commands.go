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
	Name          *string
	Description   *string
	Status        *string
	Stages        []model.WorkflowStage
	Notifications *[]model.WorkflowNotification
	Variables     *[]model.WorkflowVariable
	Triggers      *[]model.WorkflowTrigger
	Advanced      *model.WorkflowAdvanced
}

type UpdateWorkflowHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
	box       model.ValueCipher
	hooks     GitHookSync
}

type GitHookSync interface {
	Sync(ctx context.Context, wf *model.ProductWorkflow) error
}

func (h *UpdateWorkflowHandler) UseVariableCipher(box model.ValueCipher) {
	h.box = box
}

func (h *UpdateWorkflowHandler) UseGitHooks(hooks GitHookSync) {
	h.hooks = hooks
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
	if input.Notifications != nil {
		wf.Notifications = model.NormalizeNotifications(*input.Notifications)
	}
	if input.Variables != nil {
		previous := append([]model.WorkflowVariable(nil), wf.Variables...)
		variables, err := model.NormalizeVariables(*input.Variables)
		if err != nil {
			return nil, err
		}
		variables, err = model.SealVariables(h.box, previous, variables)
		if err != nil {
			return nil, err
		}
		wf.Variables = variables
	}
	if input.Triggers != nil {
		triggers, err := model.NormalizeTriggers(*input.Triggers)
		if err != nil {
			return nil, err
		}
		wf.Triggers = triggers
	}
	if input.Advanced != nil {
		wf.Advanced = model.NormalizeAdvanced(*input.Advanced)
	}
	wf.UpdatedAt = time.Now().UTC()
	if err := h.workflows.UpdateWorkflow(ctx, wf); err != nil {
		return nil, err
	}
	if h.hooks != nil {
		if err := h.hooks.Sync(ctx, wf); err != nil {
			return wf, err
		}
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
	gitLookup ProjectGitLookup
	box       model.ValueCipher
	vault     VaultReader
}

type VaultReader interface {
	Read(ctx context.Context, path string) (string, error)
}

func (h *StartWorkflowRunHandler) UseVariableCipher(box model.ValueCipher) {
	h.box = box
}

func (h *StartWorkflowRunHandler) UseVault(reader VaultReader) {
	h.vault = reader
}

type ProjectGit struct {
	GitURL     string
	ServiceURL string
	Token      string
	ProductID  kernel.ProductID
}

type ProjectGitLookup interface {
	LookupProjectGit(ctx context.Context, projectID kernel.ProjectID) (ProjectGit, error)
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
	memberID kernel.MemberID,
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
	return h.launch(ctx, wf, "manual", memberID, input, nil)
}

func (h *StartWorkflowRunHandler) launch(
	ctx context.Context,
	wf *model.ProductWorkflow,
	source string,
	actor kernel.MemberID,
	input map[string]any,
	git *engine.GitRunContext,
) (*model.ProductWorkflowRun, error) {
	if err := h.rejectIfBusy(ctx, wf); err != nil {
		return nil, err
	}
	prepared := *wf
	variables, err := h.materializeVariables(ctx, wf.Variables)
	if err != nil {
		return nil, err
	}
	prepared.Variables = variables
	runID := kernel.NewProductWorkflowRunID()
	stages, stored, trigger, err := engine.PrepareRun(&prepared, runID.String(), source, input, git)
	if err != nil {
		return nil, err
	}
	if actor != (kernel.MemberID{}) {
		trigger["started_by"] = actor.String()
	}
	run := model.NewProductWorkflowRun(wf.ID, wf.ProductID, stages)
	run.ID = runID
	if raw, err := json.Marshal(trigger); err == nil {
		run.Trigger = raw
	}
	if raw, err := json.Marshal(stored); err == nil {
		run.Input = raw
	}
	runWF := prepared
	runWF.Stages = stages
	if err := h.workflows.SaveRun(ctx, run); err != nil {
		return nil, err
	}
	if err := h.scheduler.StartRun(ctx, &runWF, run); err != nil {
		return nil, err
	}
	return h.workflows.FindRunByID(ctx, run.ID)
}

func (h *StartWorkflowRunHandler) materializeVariables(ctx context.Context, items []model.WorkflowVariable) ([]model.WorkflowVariable, error) {
	revealed, err := model.RevealVariables(h.box, items)
	if err != nil {
		return nil, err
	}
	if h.vault == nil {
		return revealed, nil
	}
	for i := range revealed {
		path := strings.TrimSpace(revealed[i].VaultPath)
		if path == "" {
			continue
		}
		value, err := h.vault.Read(ctx, path)
		if err != nil {
			return nil, kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "读取 Vault 失败: "+err.Error())
		}
		revealed[i].Value = value
	}
	return revealed, nil
}

func (h *StartWorkflowRunHandler) ProgressActiveRuns(ctx context.Context) {
	if h.scheduler == nil {
		return
	}
	runs, err := h.workflows.ListActiveRuns(ctx, 100)
	if err != nil {
		return
	}
	for _, run := range runs {
		if run == nil {
			continue
		}
		_ = h.scheduler.ProgressStuckRun(ctx, run.ID)
	}
}

func (h *StartWorkflowRunHandler) ExpireRuns(ctx context.Context) {
	if h.scheduler == nil {
		return
	}
	h.ProgressActiveRuns(ctx)
	runs, err := h.workflows.ListActiveRuns(ctx, 100)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, run := range runs {
		wf, err := h.workflows.FindWorkflowByID(ctx, run.WorkflowID)
		if err != nil || wf == nil || wf.Advanced.TimeoutMinutes <= 0 {
			continue
		}
		started := run.CreatedAt
		if run.StartedAt != nil {
			started = *run.StartedAt
		}
		if now.Sub(started) < time.Duration(wf.Advanced.TimeoutMinutes)*time.Minute {
			continue
		}
		_ = h.scheduler.ExpireRun(ctx, run)
	}
}

func (h *StartWorkflowRunHandler) rejectIfBusy(ctx context.Context, wf *model.ProductWorkflow) error {
	limit := wf.Advanced.Concurrency
	if limit <= 0 {
		return nil
	}
	count, err := h.workflows.CountActiveRuns(ctx, wf.ID)
	if err != nil {
		return err
	}
	if count >= int64(limit) {
		return domain.ErrWorkflowRunBusy
	}
	return nil
}

func slugKey(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	key = strings.ReplaceAll(key, " ", "-")
	if key == "" {
		key = "workflow-" + time.Now().UTC().Format("20060102150405")
	}
	return key
}
