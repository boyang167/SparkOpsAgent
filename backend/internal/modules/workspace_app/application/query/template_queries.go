package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListTemplatesHandler struct {
	repo         repository.TemplateRepository
	wsChecker    port.WorkspaceChecker
	memberLookup port.MemberLookup
}

func NewListTemplatesHandler(
	repo repository.TemplateRepository,
	wsChecker port.WorkspaceChecker,
	memberLookup port.MemberLookup,
) *ListTemplatesHandler {
	return &ListTemplatesHandler{repo: repo, wsChecker: wsChecker, memberLookup: memberLookup}
}

func (h *ListTemplatesHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	orgID, err := h.wsChecker.OrgIDForWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	items, err := h.repo.ListAccessible(ctx, wsID, orgID)
	if err != nil {
		return nil, err
	}
	creators, err := h.loadCreators(ctx, items)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, TemplateViewToMap(wsID, item, creators))
	}
	return out, nil
}

type GetTemplateHandler struct {
	repo         repository.TemplateRepository
	wsChecker    port.WorkspaceChecker
	memberLookup port.MemberLookup
}

func NewGetTemplateHandler(
	repo repository.TemplateRepository,
	wsChecker port.WorkspaceChecker,
	memberLookup port.MemberLookup,
) *GetTemplateHandler {
	return &GetTemplateHandler{repo: repo, wsChecker: wsChecker, memberLookup: memberLookup}
}

func (h *GetTemplateHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, templateID kernel.TemplateID) (map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	orgID, err := h.wsChecker.OrgIDForWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	view, err := h.repo.FindAccessible(ctx, wsID, orgID, templateID)
	if err != nil {
		return nil, err
	}
	creators, err := h.memberLookup.FindByIDs(ctx, []kernel.MemberID{view.Template.CreatedBy})
	if err != nil {
		return nil, err
	}
	return TemplateViewToMap(wsID, view, creators), nil
}

func (h *ListTemplatesHandler) loadCreators(ctx context.Context, items []*model.TemplateView) (map[kernel.MemberID]port.MemberSummary, error) {
	ids := make([]kernel.MemberID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Template.CreatedBy)
	}
	return h.memberLookup.FindByIDs(ctx, ids)
}

func EnrichTemplateMap(ctx context.Context, lookup port.MemberLookup, t *model.Template) (map[string]any, error) {
	creators, err := lookup.FindByIDs(ctx, []kernel.MemberID{t.CreatedBy})
	if err != nil {
		return nil, err
	}
	return TemplateToMap(t, creators), nil
}

func TemplateViewToMap(currentWS kernel.WorkspaceID, view *model.TemplateView, creators map[kernel.MemberID]port.MemberSummary) map[string]any {
	m := TemplateToMap(view.Template, creators)
	m["workspace_id"] = view.Template.WorkspaceID.String()
	if view.WorkspaceName != "" {
		m["workspace_name"] = view.WorkspaceName
	}
	m["shared"] = view.Template.WorkspaceID != currentWS
	m["editable"] = view.Template.WorkspaceID == currentWS
	return m
}

func TemplateToMap(t *model.Template, creators map[kernel.MemberID]port.MemberSummary) map[string]any {
	m := map[string]any{
		"id": t.ID.String(), "name": t.Name.String(), "kind": t.Kind,
		"language": t.Language, "detail": t.Detail, "rules": t.Rules,
		"status": string(t.Status), "is_public": t.IsPublic,
		"created_by": t.CreatedBy.String(),
		"created_at": t.CreatedAt, "updated_at": t.UpdatedAt,
	}
	if c, ok := creators[t.CreatedBy]; ok {
		m["creator"] = map[string]any{
			"id": c.ID.String(), "email": c.Email, "display_name": c.DisplayName,
		}
	}
	return m
}

func ListAccessibleTemplates(
	ctx context.Context,
	repo repository.TemplateRepository,
	wsChecker port.WorkspaceChecker,
	wsID kernel.WorkspaceID,
) ([]*model.Template, error) {
	orgID, err := wsChecker.OrgIDForWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	views, err := repo.ListAccessible(ctx, wsID, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Template, 0, len(views))
	for _, view := range views {
		out = append(out, view.Template)
	}
	return out, nil
}
