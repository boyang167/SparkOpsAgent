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
	items, err := h.repo.ListByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	creators, err := h.loadCreators(ctx, items)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		out = append(out, TemplateToMap(t, creators))
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
	t, err := h.repo.FindByID(ctx, wsID, templateID)
	if err != nil {
		return nil, err
	}
	creators, err := h.memberLookup.FindByIDs(ctx, []kernel.MemberID{t.CreatedBy})
	if err != nil {
		return nil, err
	}
	return TemplateToMap(t, creators), nil
}

func (h *ListTemplatesHandler) loadCreators(ctx context.Context, items []*model.Template) (map[kernel.MemberID]port.MemberSummary, error) {
	ids := make([]kernel.MemberID, 0, len(items))
	for _, t := range items {
		ids = append(ids, t.CreatedBy)
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

type ListDockerfilesHandler struct {
	repo      repository.MarketRepository
	wsChecker port.WorkspaceChecker
}

func NewListDockerfilesHandler(repo repository.MarketRepository, wsChecker port.WorkspaceChecker) *ListDockerfilesHandler {
	return &ListDockerfilesHandler{repo: repo, wsChecker: wsChecker}
}

func (h *ListDockerfilesHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	items, err := h.repo.ListDockerfilesByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, df := range items {
		out = append(out, map[string]any{
			"id": df.ID.String(), "name": df.Name.String(), "language": df.Language,
			"description": df.Description, "detail": df.Detail, "dockerfile": df.Dockerfile,
			"version": df.Version, "download_count": df.DownloadCount, "score": df.Score,
			"created_at": df.CreatedAt,
		})
	}
	return out, nil
}
