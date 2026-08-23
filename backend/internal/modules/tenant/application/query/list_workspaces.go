package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceView struct {
	ID          string `json:"id"`
	OrgID       string `json:"org_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	MaxAgents   int    `json:"max_agents"`
	MaxProjects int    `json:"max_projects"`
	IsMember    bool   `json:"is_member"`
	ReadOnly    bool   `json:"read_only"`
}

type ListWorkspacesHandler struct {
	repo        repository.WorkspaceRepository
	systemRoles authz.SystemRoleChecker
}

func NewListWorkspacesHandler(
	repo repository.WorkspaceRepository,
	systemRoles authz.SystemRoleChecker,
) *ListWorkspacesHandler {
	return &ListWorkspacesHandler{repo: repo, systemRoles: systemRoles}
}

func (h *ListWorkspacesHandler) Handle(ctx context.Context, memberID kernel.MemberID) ([]WorkspaceView, error) {
	isSuper, err := h.systemRoles.IsSuperAdmin(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if isSuper {
		return h.listAllForSuperAdmin(ctx, memberID)
	}
	return h.listByMember(ctx, memberID)
}

func (h *ListWorkspacesHandler) listByMember(ctx context.Context, memberID kernel.MemberID) ([]WorkspaceView, error) {
	items, err := h.repo.ListByMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceView, 0, len(items))
	for _, ws := range items {
		out = append(out, toWorkspaceView(ws, true, false))
	}
	return out, nil
}

func (h *ListWorkspacesHandler) listAllForSuperAdmin(ctx context.Context, memberID kernel.MemberID) ([]WorkspaceView, error) {
	items, err := h.repo.ListAllWithMembership(ctx, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceView, 0, len(items))
	for _, item := range items {
		readOnly := !item.IsMember
		out = append(out, toWorkspaceView(item.Workspace, item.IsMember, readOnly))
	}
	return out, nil
}

func toWorkspaceView(ws *model.Workspace, isMember, readOnly bool) WorkspaceView {
	return WorkspaceView{
		ID: ws.ID.String(), OrgID: ws.OrgID.String(),
		Name: ws.Name, Slug: ws.Slug,
		MaxAgents: ws.Quota.MaxAgents, MaxProjects: ws.Quota.MaxProjects,
		IsMember: isMember, ReadOnly: readOnly,
	}
}
