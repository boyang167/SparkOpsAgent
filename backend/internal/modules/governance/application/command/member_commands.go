package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type InviteWorkspaceMemberHandler struct {
	identity *acl.IdentityACL
	tenant   *acl.TenantACL
}

func NewInviteWorkspaceMemberHandler(identity *acl.IdentityACL, tenant *acl.TenantACL) *InviteWorkspaceMemberHandler {
	return &InviteWorkspaceMemberHandler{identity: identity, tenant: tenant}
}

type InviteMemberResult struct {
	MemberID    kernel.MemberID
	Email       string
	DisplayName string
}

func (h *InviteWorkspaceMemberHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, email string) (*InviteMemberResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, domain.ErrMemberNotFound
	}
	member, err := h.identity.FindMemberByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	exists, err := h.tenant.MemberInWorkspace(ctx, wsID, member.MemberID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrMemberAlreadyInWorkspace
	}
	if err := h.tenant.AddWorkspaceMember(ctx, wsID, member.MemberID, "member"); err != nil {
		return nil, err
	}
	return &InviteMemberResult{
		MemberID: member.MemberID, Email: member.Email, DisplayName: member.DisplayName,
	}, nil
}

type RemoveRoleBindingHandler struct {
	roles    repository.RoleRepository
	bindings repository.BindingRepository
	sync     *service.APIPolicySync
}

func NewRemoveRoleBindingHandler(
	roles repository.RoleRepository,
	bindings repository.BindingRepository,
	sync *service.APIPolicySync,
) *RemoveRoleBindingHandler {
	return &RemoveRoleBindingHandler{roles: roles, bindings: bindings, sync: sync}
}

func (h *RemoveRoleBindingHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	roleID kernel.RoleID,
	memberID kernel.MemberID,
) error {
	role, err := h.roles.FindByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role.WorkspaceID != workspaceID {
		return domain.ErrRoleNotFound
	}
	if err := h.bindings.Delete(ctx, roleID, memberID); err != nil {
		return err
	}
	if h.sync != nil {
		return h.sync.SyncMemberRole(ctx, workspaceID, roleID, memberID, false)
	}
	return nil
}
