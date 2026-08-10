package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceView struct {
	ID          string `json:"id"`
	OrgID       string `json:"org_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	MaxAgents   int    `json:"max_agents"`
	MaxProjects int    `json:"max_projects"`
}

type ListWorkspacesHandler struct {
	repo repository.WorkspaceRepository
}

func NewListWorkspacesHandler(repo repository.WorkspaceRepository) *ListWorkspacesHandler {
	return &ListWorkspacesHandler{repo: repo}
}

func (h *ListWorkspacesHandler) Handle(ctx context.Context, memberID kernel.MemberID) ([]WorkspaceView, error) {
	items, err := h.repo.ListByMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceView, 0, len(items))
	for _, ws := range items {
		out = append(out, WorkspaceView{
			ID: ws.ID.String(), OrgID: ws.OrgID.String(),
			Name: ws.Name, Slug: ws.Slug,
			MaxAgents: ws.Quota.MaxAgents, MaxProjects: ws.Quota.MaxProjects,
		})
	}
	return out, nil
}
