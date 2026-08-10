package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MemberRoleView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type WorkspaceMemberView struct {
	MemberID    string           `json:"member_id"`
	Email       string           `json:"email"`
	DisplayName string           `json:"display_name"`
	Roles       []MemberRoleView `json:"roles"`
}

type ListWorkspaceMembersHandler struct {
	members  *acl.MemberACL
	bindings repository.BindingRepository
}

func NewListWorkspaceMembersHandler(members *acl.MemberACL, bindings repository.BindingRepository) *ListWorkspaceMembersHandler {
	return &ListWorkspaceMembersHandler{members: members, bindings: bindings}
}

func (h *ListWorkspaceMembersHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID) ([]WorkspaceMemberView, error) {
	members, err := h.members.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceMemberView, 0, len(members))
	for _, member := range members {
		roles, err := h.bindings.ListRolesForMember(ctx, workspaceID, member.MemberID)
		if err != nil {
			return nil, err
		}
		roleViews := make([]MemberRoleView, 0, len(roles))
		for _, role := range roles {
			roleViews = append(roleViews, MemberRoleView{
				ID: role.ID.String(), Name: role.Name, Permissions: role.Permissions,
			})
		}
		out = append(out, WorkspaceMemberView{
			MemberID:    member.MemberID.String(),
			Email:       member.Email,
			DisplayName: member.DisplayName,
			Roles:       roleViews,
		})
	}
	return out, nil
}

type ListMemberRolesHandler struct {
	bindings repository.BindingRepository
}

func NewListMemberRolesHandler(bindings repository.BindingRepository) *ListMemberRolesHandler {
	return &ListMemberRolesHandler{bindings: bindings}
}

func (h *ListMemberRolesHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) ([]MemberRoleView, error) {
	roles, err := h.bindings.ListRolesForMember(ctx, workspaceID, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberRoleView, 0, len(roles))
	for _, role := range roles {
		out = append(out, MemberRoleView{
			ID: role.ID.String(), Name: role.Name, Permissions: role.Permissions,
		})
	}
	return out, nil
}
