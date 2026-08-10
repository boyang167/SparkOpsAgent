package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrOrganizationNotFound = kernel.NewDomainError("TENANT_ORG_NOT_FOUND", "organization not found")
	ErrWorkspaceNotFound    = kernel.NewDomainError("TENANT_WORKSPACE_NOT_FOUND", "workspace not found")
	ErrSlugAlreadyExists    = kernel.NewDomainError("TENANT_SLUG_EXISTS", "slug already exists")
	ErrQuotaExceeded        = kernel.NewDomainError("TENANT_QUOTA_EXCEEDED", "quota exceeded")
	ErrOrgAccessDenied      = kernel.NewDomainError("TENANT_ORG_ACCESS_DENIED", "organization access denied")
	ErrNotWorkspaceMember   = kernel.NewDomainError("TENANT_NOT_WORKSPACE_MEMBER", "workspace membership required")
	ErrOrgMemberNotFound    = kernel.NewDomainError("TENANT_ORG_MEMBER_NOT_FOUND", "organization member not found")
	ErrOrgMemberExists      = kernel.NewDomainError("TENANT_ORG_MEMBER_EXISTS", "member already in organization")
	ErrCannotRemoveOwner    = kernel.NewDomainError("TENANT_ORG_CANNOT_REMOVE_OWNER", "cannot remove organization owner")
	ErrInvalidOrgRole       = kernel.NewDomainError("TENANT_ORG_INVALID_ROLE", "invalid organization role")
	ErrMemberNotFound       = kernel.NewDomainError("TENANT_MEMBER_NOT_FOUND", "member not found")
	ErrOnlyOwnerCanTransfer = kernel.NewDomainError("TENANT_ORG_ONLY_OWNER_TRANSFER", "only owner can transfer ownership")
	ErrNewOwnerNotMember    = kernel.NewDomainError("TENANT_ORG_NEW_OWNER_NOT_MEMBER", "new owner must be an organization member")
	ErrCannotTransferToSelf = kernel.NewDomainError("TENANT_ORG_CANNOT_TRANSFER_SELF", "cannot transfer ownership to yourself")
	ErrInvalidOrgStatus     = kernel.NewDomainError("TENANT_ORG_INVALID_STATUS", "invalid organization status")
)

const (
	OrgStatusActive    = "active"
	OrgStatusSuspended = "suspended"
)

const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"
)
