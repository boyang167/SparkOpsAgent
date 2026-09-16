package query

import (
	"context"
	"encoding/json"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/keyval"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListWorkflowTemplatesHandler struct {
	repo      repository.WorkflowTemplateRepository
	wsChecker port.WorkspaceChecker
}

func NewListWorkflowTemplatesHandler(repo repository.WorkflowTemplateRepository, wsChecker port.WorkspaceChecker) *ListWorkflowTemplatesHandler {
	return &ListWorkflowTemplatesHandler{repo: repo, wsChecker: wsChecker}
}

func (h *ListWorkflowTemplatesHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	items, err := h.repo.ListForWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, templateToMap(item))
	}
	return out, nil
}

type GetWorkflowTemplateHandler struct {
	repo      repository.WorkflowTemplateRepository
	wsChecker port.WorkspaceChecker
}

func NewGetWorkflowTemplateHandler(repo repository.WorkflowTemplateRepository, wsChecker port.WorkspaceChecker) *GetWorkflowTemplateHandler {
	return &GetWorkflowTemplateHandler{repo: repo, wsChecker: wsChecker}
}

func (h *GetWorkflowTemplateHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, templateID kernel.WorkflowTemplateID) (map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	tpl, err := h.repo.FindByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if tpl.Scope == model.TemplateScopeWorkspace && tpl.WorkspaceID != nil && *tpl.WorkspaceID != wsID {
		return nil, domain.ErrWorkflowTemplateNotFound
	}
	return templateToMap(tpl), nil
}

type PreviewWorkflowTemplateHandler struct {
	repo repository.WorkflowTemplateRepository
}

func NewPreviewWorkflowTemplateHandler(repo repository.WorkflowTemplateRepository) *PreviewWorkflowTemplateHandler {
	return &PreviewWorkflowTemplateHandler{repo: repo}
}

func (h *PreviewWorkflowTemplateHandler) Handle(ctx context.Context, templateID kernel.WorkflowTemplateID, overrides map[string]string) (map[string]any, error) {
	tpl, err := h.repo.FindByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	merged := keyval.MergeKeyVals(tpl.KeyVals, overrides)
	resolved, err := keyval.Resolve(tpl.Definition, merged)
	if err != nil {
		return nil, err
	}
	var parsed any
	_ = json.Unmarshal(resolved, &parsed)
	return map[string]any{
		"template_id": tpl.ID.String(),
		"key_vals":    merged,
		"definition":  parsed,
	}, nil
}

func templateToMap(tpl *model.WorkflowTemplate) map[string]any {
	out := map[string]any{
		"id": tpl.ID.String(), "scope": tpl.Scope, "name": tpl.Name, "key": tpl.Key,
		"description": tpl.Description, "category": tpl.Category, "product_types": tpl.ProductTypes,
		"status": tpl.Status, "version": tpl.Version, "key_vals": tpl.KeyVals,
		"created_at": tpl.CreatedAt, "updated_at": tpl.UpdatedAt,
	}
	if tpl.WorkspaceID != nil {
		out["workspace_id"] = tpl.WorkspaceID.String()
	}
	if len(tpl.Definition) > 0 {
		var def any
		_ = json.Unmarshal(tpl.Definition, &def)
		out["definition"] = def
	}
	if len(tpl.SlotSchema) > 0 {
		var slots any
		_ = json.Unmarshal(tpl.SlotSchema, &slots)
		out["slot_schema"] = slots
	}
	return out
}
