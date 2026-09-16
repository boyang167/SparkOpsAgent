package command

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateWorkflowTemplateInput struct {
	Name        string
	Key         string
	Description string
	Definition  json.RawMessage
}

type CreateWorkflowTemplateHandler struct {
	templates repository.WorkflowTemplateRepository
	wsChecker port.WorkspaceChecker
}

func NewCreateWorkflowTemplateHandler(templates repository.WorkflowTemplateRepository, wsChecker port.WorkspaceChecker) *CreateWorkflowTemplateHandler {
	return &CreateWorkflowTemplateHandler{templates: templates, wsChecker: wsChecker}
}

func (h *CreateWorkflowTemplateHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, input CreateWorkflowTemplateInput) (*model.WorkflowTemplate, error) {
	if err := ensureWorkspaceMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	key := strings.TrimSpace(input.Key)
	if name == "" || key == "" {
		return nil, domain.ErrInvalidWorkflowInput
	}
	if err := validateTemplateDefinition(input.Definition); err != nil {
		return nil, err
	}
	if _, err := h.templates.FindByKey(ctx, key); err == nil {
		return nil, domain.ErrWorkflowTemplateKeyExists
	}
	tpl := model.NewWorkspaceTemplate(wsID, name, key, strings.TrimSpace(input.Description), input.Definition, memberID)
	if err := h.templates.Save(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

type UpdateWorkflowTemplateInput struct {
	Name        string
	Description string
	Definition  json.RawMessage
}

type UpdateWorkflowTemplateHandler struct {
	templates repository.WorkflowTemplateRepository
	wsChecker port.WorkspaceChecker
}

func NewUpdateWorkflowTemplateHandler(templates repository.WorkflowTemplateRepository, wsChecker port.WorkspaceChecker) *UpdateWorkflowTemplateHandler {
	return &UpdateWorkflowTemplateHandler{templates: templates, wsChecker: wsChecker}
}

func (h *UpdateWorkflowTemplateHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, templateID kernel.WorkflowTemplateID, input UpdateWorkflowTemplateInput) (*model.WorkflowTemplate, error) {
	if err := ensureWorkspaceMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
	}
	tpl, err := h.templates.FindByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if err := tpl.EnsureWorkspaceEditable(wsID); err != nil {
		return nil, err
	}
	if len(input.Definition) > 0 {
		if err := validateTemplateDefinition(input.Definition); err != nil {
			return nil, err
		}
	}
	tpl.UpdateContent(strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), input.Definition)
	if err := h.templates.Update(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

type PublishWorkflowTemplateHandler struct {
	templates repository.WorkflowTemplateRepository
	wsChecker   port.WorkspaceChecker
}

func NewPublishWorkflowTemplateHandler(templates repository.WorkflowTemplateRepository, wsChecker port.WorkspaceChecker) *PublishWorkflowTemplateHandler {
	return &PublishWorkflowTemplateHandler{templates: templates, wsChecker: wsChecker}
}

func (h *PublishWorkflowTemplateHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, templateID kernel.WorkflowTemplateID) (*model.WorkflowTemplate, error) {
	if err := ensureWorkspaceMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
	}
	tpl, err := h.templates.FindByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if err := tpl.EnsureWorkspaceEditable(wsID); err != nil {
		return nil, err
	}
	if err := tpl.Publish(); err != nil {
		return nil, err
	}
	if err := h.templates.Update(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

func ensureWorkspaceMember(ctx context.Context, checker port.WorkspaceChecker, memberID kernel.MemberID, wsID kernel.WorkspaceID) error {
	ok, err := checker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWorkspaceAccessDenied
	}
	return nil
}

func validateTemplateDefinition(definition json.RawMessage) error {
	if len(definition) == 0 {
		return domain.ErrInvalidWorkflowInput
	}
	var wrapper struct {
		Stages []json.RawMessage `json:"stages"`
	}
	if err := json.Unmarshal(definition, &wrapper); err != nil {
		return domain.ErrInvalidWorkflowInput
	}
	if len(wrapper.Stages) == 0 {
		return domain.ErrInvalidWorkflowInput
	}
	return nil
}
