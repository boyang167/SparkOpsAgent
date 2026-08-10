package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectMemberView struct {
	MemberID    string `json:"member_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	JoinedAt    string `json:"joined_at"`
}

type ListProjectMembersHandler struct {
	projectRepo repository.ProjectRepository
	memberRepo  repository.ProjectMemberRepository
}

func NewListProjectMembersHandler(projectRepo repository.ProjectRepository, memberRepo repository.ProjectMemberRepository) *ListProjectMembersHandler {
	return &ListProjectMembersHandler{projectRepo: projectRepo, memberRepo: memberRepo}
}

func (h *ListProjectMembersHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]ProjectMemberView, error) {
	project, err := h.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.WorkspaceID != wsID {
		return nil, domain.ErrProjectNotFound
	}
	records, err := h.memberRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectMemberView, 0, len(records))
	for _, r := range records {
		out = append(out, ProjectMemberView{
			MemberID: r.MemberID.String(), Email: r.Email, DisplayName: r.DisplayName,
			Role: r.Role, JoinedAt: r.JoinedAt,
		})
	}
	return out, nil
}
