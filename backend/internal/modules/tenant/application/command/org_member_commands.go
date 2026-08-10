package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type InviteOrgMemberHandler struct {
	repo     repository.OrganizationRepository
	identity *acl.IdentityACL
	audit    audit.Logger
}

func NewInviteOrgMemberHandler(repo repository.OrganizationRepository, identity *acl.IdentityACL, auditLogger audit.Logger) *InviteOrgMemberHandler {
	return &InviteOrgMemberHandler{repo: repo, identity: identity, audit: auditLogger}
}

type InviteOrgMemberResult struct {
	MemberID    kernel.MemberID
	Email       string
	DisplayName string
	Role        string
}

func (h *InviteOrgMemberHandler) Handle(
	ctx context.Context,
	orgID kernel.OrgID,
	actorID kernel.MemberID,
	email, role string,
) (*InviteOrgMemberResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	role = strings.TrimSpace(role)
	if role == "" {
		role = domain.OrgRoleMember
	}
	if !isValidOrgRole(role) || role == domain.OrgRoleOwner {
		return nil, domain.ErrInvalidOrgRole
	}
	actorRole, err := h.repo.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return nil, domain.ErrOrgAccessDenied
	}
	if err := canManageOrgMember(actorRole, role); err != nil {
		return nil, err
	}
	member, err := h.identity.FindMemberByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	exists, err := h.repo.IsMember(ctx, orgID, member.MemberID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrOrgMemberExists
	}
	if err := h.repo.AddMember(ctx, orgID, member.MemberID, role); err != nil {
		return nil, err
	}
	logAudit(h.audit, ctx, orgAuditEntry(actorID, audit.ActionOrgMemberInvite, member.MemberID.String(), orgID, map[string]any{
		"email": member.Email, "role": role,
	}))
	return &InviteOrgMemberResult{
		MemberID: member.MemberID, Email: member.Email,
		DisplayName: member.DisplayName, Role: role,
	}, nil
}

type UpdateOrgMemberRoleHandler struct {
	repo  repository.OrganizationRepository
	audit audit.Logger
}

func NewUpdateOrgMemberRoleHandler(repo repository.OrganizationRepository, auditLogger audit.Logger) *UpdateOrgMemberRoleHandler {
	return &UpdateOrgMemberRoleHandler{repo: repo, audit: auditLogger}
}

func (h *UpdateOrgMemberRoleHandler) Handle(
	ctx context.Context,
	orgID kernel.OrgID,
	actorID, targetID kernel.MemberID,
	role string,
) error {
	role = strings.TrimSpace(role)
	if !isValidOrgRole(role) {
		return domain.ErrInvalidOrgRole
	}
	actorRole, err := h.repo.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return domain.ErrOrgAccessDenied
	}
	targetRole, err := h.repo.GetMemberRole(ctx, orgID, targetID)
	if err != nil {
		return err
	}
	if targetRole == domain.OrgRoleOwner {
		return domain.ErrCannotRemoveOwner
	}
	if err := canManageOrgMember(actorRole, role); err != nil {
		return err
	}
	if targetRole == role {
		return nil
	}
	if err := h.repo.UpdateMemberRole(ctx, orgID, targetID, role); err != nil {
		return err
	}
	logAudit(h.audit, ctx, orgAuditEntry(actorID, audit.ActionOrgMemberRoleUpdate, targetID.String(), orgID, map[string]any{
		"from_role": targetRole, "to_role": role,
	}))
	return nil
}

type RemoveOrgMemberHandler struct {
	repo  repository.OrganizationRepository
	audit audit.Logger
}

func NewRemoveOrgMemberHandler(repo repository.OrganizationRepository, auditLogger audit.Logger) *RemoveOrgMemberHandler {
	return &RemoveOrgMemberHandler{repo: repo, audit: auditLogger}
}

func (h *RemoveOrgMemberHandler) Handle(ctx context.Context, orgID kernel.OrgID, actorID, targetID kernel.MemberID) error {
	actorRole, err := h.repo.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return domain.ErrOrgAccessDenied
	}
	if actorRole != domain.OrgRoleOwner && actorRole != domain.OrgRoleAdmin {
		return domain.ErrOrgAccessDenied
	}
	targetRole, err := h.repo.GetMemberRole(ctx, orgID, targetID)
	if err != nil {
		return err
	}
	if targetRole == domain.OrgRoleOwner {
		return domain.ErrCannotRemoveOwner
	}
	if actorRole == domain.OrgRoleAdmin && targetRole == domain.OrgRoleAdmin {
		return domain.ErrOrgAccessDenied
	}
	if err := h.repo.RemoveMember(ctx, orgID, targetID); err != nil {
		return err
	}
	logAudit(h.audit, ctx, orgAuditEntry(actorID, audit.ActionOrgMemberRemove, targetID.String(), orgID, map[string]any{
		"removed_role": targetRole,
	}))
	return nil
}

func isValidOrgRole(role string) bool {
	return role == domain.OrgRoleOwner || role == domain.OrgRoleAdmin || role == domain.OrgRoleMember
}

func canManageOrgMember(actorRole, targetRole string) error {
	switch actorRole {
	case domain.OrgRoleOwner:
		return nil
	case domain.OrgRoleAdmin:
		if targetRole == domain.OrgRoleAdmin || targetRole == domain.OrgRoleOwner {
			return domain.ErrOrgAccessDenied
		}
		return nil
	default:
		return domain.ErrOrgAccessDenied
	}
}
