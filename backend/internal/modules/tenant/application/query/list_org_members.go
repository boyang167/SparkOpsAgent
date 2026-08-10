package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OrgMemberView struct {
	MemberID    string `json:"member_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	JoinedAt    string `json:"joined_at"`
}

type ListOrgMembersHandler struct {
	repo repository.OrganizationRepository
}

func NewListOrgMembersHandler(repo repository.OrganizationRepository) *ListOrgMembersHandler {
	return &ListOrgMembersHandler{repo: repo}
}

func (h *ListOrgMembersHandler) Handle(ctx context.Context, orgID kernel.OrgID) ([]OrgMemberView, error) {
	members, err := h.repo.ListMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]OrgMemberView, 0, len(members))
	for _, m := range members {
		out = append(out, OrgMemberView{
			MemberID: m.MemberID.String(), Email: m.Email,
			DisplayName: m.DisplayName, Role: m.Role, JoinedAt: m.JoinedAt,
		})
	}
	return out, nil
}
