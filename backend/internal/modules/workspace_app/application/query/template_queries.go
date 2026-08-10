package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListTemplatesHandler struct {
	repo      repository.TemplateRepository
	wsChecker port.WorkspaceChecker
}

func NewListTemplatesHandler(repo repository.TemplateRepository, wsChecker port.WorkspaceChecker) *ListTemplatesHandler {
	return &ListTemplatesHandler{repo: repo, wsChecker: wsChecker}
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
	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		out = append(out, map[string]any{
			"id": t.ID.String(), "name": t.Name.String(), "kind": t.Kind,
			"language": t.Language, "detail": t.Detail, "rules": t.Rules,
			"created_at": t.CreatedAt,
		})
	}
	return out, nil
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
