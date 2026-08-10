package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreatePlatformOrganizationHandler struct {
	repo     repository.OrganizationRepository
	identity *acl.IdentityACL
	audit    audit.Logger
}

func NewCreatePlatformOrganizationHandler(
	repo repository.OrganizationRepository,
	identity *acl.IdentityACL,
	auditLogger audit.Logger,
) *CreatePlatformOrganizationHandler {
	return &CreatePlatformOrganizationHandler{repo: repo, identity: identity, audit: auditLogger}
}

type CreatePlatformOrganizationInput struct {
	Name       string
	Slug       string
	OwnerEmail string
}

func (h *CreatePlatformOrganizationHandler) Handle(
	ctx context.Context,
	actorID kernel.MemberID,
	input CreatePlatformOrganizationInput,
) (*model.Organization, error) {
	ownerEmail := strings.TrimSpace(strings.ToLower(input.OwnerEmail))
	owner, err := h.identity.FindMemberByEmail(ctx, ownerEmail)
	if err != nil {
		return nil, err
	}
	org := model.NewOrganization(input.Name, input.Slug)
	exists, err := h.repo.ExistsBySlug(ctx, org.Slug)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrSlugAlreadyExists
	}
	if err := h.repo.Save(ctx, org); err != nil {
		return nil, err
	}
	if err := h.repo.AddMember(ctx, org.ID, owner.MemberID, domain.OrgRoleOwner); err != nil {
		return nil, err
	}
	logAudit(h.audit, ctx, orgAuditEntry(actorID, audit.ActionPlatformOrgCreate, org.ID.String(), org.ID, map[string]any{
		"name": org.Name, "slug": org.Slug, "owner_email": owner.Email, "owner_id": owner.MemberID.String(),
	}))
	return org, nil
}

type UpdateOrganizationStatusHandler struct {
	repo  repository.OrganizationRepository
	audit audit.Logger
}

func NewUpdateOrganizationStatusHandler(repo repository.OrganizationRepository, auditLogger audit.Logger) *UpdateOrganizationStatusHandler {
	return &UpdateOrganizationStatusHandler{repo: repo, audit: auditLogger}
}

func (h *UpdateOrganizationStatusHandler) Handle(ctx context.Context, actorID kernel.MemberID, orgID kernel.OrgID, status string) error {
	status = strings.TrimSpace(status)
	if status != domain.OrgStatusActive && status != domain.OrgStatusSuspended {
		return domain.ErrInvalidOrgStatus
	}
	if _, err := h.repo.FindByID(ctx, orgID); err != nil {
		return err
	}
	if err := h.repo.UpdateStatus(ctx, orgID, status); err != nil {
		return err
	}
	logAudit(h.audit, ctx, orgAuditEntry(actorID, audit.ActionOrgStatusUpdate, orgID.String(), orgID, map[string]any{
		"status": status,
	}))
	return nil
}

type TransferOrgOwnerHandler struct {
	repo     repository.OrganizationRepository
	identity *acl.IdentityACL
	audit    audit.Logger
}

func NewTransferOrgOwnerHandler(
	repo repository.OrganizationRepository,
	identity *acl.IdentityACL,
	auditLogger audit.Logger,
) *TransferOrgOwnerHandler {
	return &TransferOrgOwnerHandler{repo: repo, identity: identity, audit: auditLogger}
}

type TransferOrgOwnerResult struct {
	PreviousOwnerID kernel.MemberID
	NewOwnerID      kernel.MemberID
	NewOwnerEmail   string
}

func (h *TransferOrgOwnerHandler) Handle(
	ctx context.Context,
	orgID kernel.OrgID,
	actorID kernel.MemberID,
	newOwnerEmail string,
	bySuperAdmin bool,
) (*TransferOrgOwnerResult, error) {
	if !bySuperAdmin {
		role, err := h.repo.GetMemberRole(ctx, orgID, actorID)
		if err != nil || role != domain.OrgRoleOwner {
			return nil, domain.ErrOnlyOwnerCanTransfer
		}
	}
	newOwnerEmail = strings.TrimSpace(strings.ToLower(newOwnerEmail))
	newOwner, err := h.identity.FindMemberByEmail(ctx, newOwnerEmail)
	if err != nil {
		return nil, err
	}
	if actorID == newOwner.MemberID && !bySuperAdmin {
		return nil, domain.ErrCannotTransferToSelf
	}
	currentOwnerRole, err := h.repo.GetMemberRole(ctx, orgID, actorID)
	if err != nil && !bySuperAdmin {
		return nil, err
	}
	fromID := actorID
	if bySuperAdmin {
		members, err := h.repo.ListMembers(ctx, orgID)
		if err != nil {
			return nil, err
		}
		found := false
		for _, m := range members {
			if m.Role == domain.OrgRoleOwner {
				fromID = m.MemberID
				found = true
				break
			}
		}
		if !found {
			return nil, domain.ErrOrgMemberNotFound
		}
	} else if currentOwnerRole != domain.OrgRoleOwner {
		return nil, domain.ErrOnlyOwnerCanTransfer
	}
	if err != nil {
		return nil, err
	}
	isMember, err := h.repo.IsMember(ctx, orgID, newOwner.MemberID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		if err := h.repo.AddMember(ctx, orgID, newOwner.MemberID, domain.OrgRoleMember); err != nil {
			return nil, err
		}
	}
	if err := h.repo.TransferOwner(ctx, orgID, fromID, newOwner.MemberID); err != nil {
		return nil, err
	}
	logAudit(h.audit, ctx, orgAuditEntry(actorID, audit.ActionOrgOwnerTransfer, orgID.String(), orgID, map[string]any{
		"from_member_id": fromID.String(),
		"to_member_id":   newOwner.MemberID.String(),
		"to_email":       newOwner.Email,
		"by_super_admin": bySuperAdmin,
	}))
	return &TransferOrgOwnerResult{
		PreviousOwnerID: fromID,
		NewOwnerID:      newOwner.MemberID,
		NewOwnerEmail:   newOwner.Email,
	}, nil
}
