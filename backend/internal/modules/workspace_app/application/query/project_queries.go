package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type ListProjectsHandler struct {
	repo       repository.ProjectRepository
	memberRepo repository.ProjectMemberRepository
	wsChecker  port.WorkspaceChecker
	governance authz.PermissionChecker
}

func NewListProjectsHandler(
	repo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	wsChecker port.WorkspaceChecker,
	governance authz.PermissionChecker,
) *ListProjectsHandler {
	return &ListProjectsHandler{repo: repo, memberRepo: memberRepo, wsChecker: wsChecker, governance: governance}
}

func (h *ListProjectsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]ProjectView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	projects, err := h.repo.ListByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if h.governance != nil {
		isAdmin, err := h.governance.HasPermission(ctx, wsID, memberID, authz.PermGovernanceAdmin)
		if err != nil {
			return nil, err
		}
		if isAdmin {
			return toViews(projects), nil
		}
	}
	allowedIDs, err := h.memberRepo.ListProjectIDsForMember(ctx, wsID, memberID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(allowedIDs))
	for _, id := range allowedIDs {
		allowed[id.String()] = struct{}{}
	}
	filtered := make([]*model.Project, 0, len(projects))
	for _, p := range projects {
		if _, ok := allowed[p.ID.String()]; ok {
			filtered = append(filtered, p)
		}
	}
	return toViews(filtered), nil
}

type GetProjectHandler struct {
	repo      repository.ProjectRepository
	wsChecker port.WorkspaceChecker
}

func NewGetProjectHandler(repo repository.ProjectRepository, wsChecker port.WorkspaceChecker) *GetProjectHandler {
	return &GetProjectHandler{repo: repo, wsChecker: wsChecker}
}

func (h *GetProjectHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, projectID kernel.ProjectID) (*ProjectView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	project, err := h.repo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.WorkspaceID != wsID {
		return nil, domain.ErrProjectNotFound
	}
	if project.Status != model.ProjectStatusActive {
		return nil, domain.ErrProjectNotFound
	}
	view := toView(project)
	return &view, nil
}

func toViews(projects []*model.Project) []ProjectView {
	out := make([]ProjectView, 0, len(projects))
	for _, p := range projects {
		out = append(out, toView(p))
	}
	return out
}

func toView(p *model.Project) ProjectView {
	return ProjectView{
		ID:          p.ID.String(),
		Name:        p.Name.String(),
		Description: p.Description,
		Status:      string(p.Status),
	}
}
