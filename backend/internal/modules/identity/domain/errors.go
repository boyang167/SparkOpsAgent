package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidCredentials = kernel.NewDomainError("IDENTITY_INVALID_CREDENTIALS", "invalid email or password")
	ErrMemberDisabled       = kernel.NewDomainError("IDENTITY_MEMBER_DISABLED", "member is disabled")
	ErrEmailAlreadyExists   = kernel.NewDomainError("IDENTITY_EMAIL_EXISTS", "email already registered")
	ErrMemberNotFound       = kernel.NewDomainError("IDENTITY_MEMBER_NOT_FOUND", "member not found")
	ErrInvalidProfile       = kernel.NewDomainError("IDENTITY_INVALID_PROFILE", "invalid profile data")
	ErrCannotModifySelf     = kernel.NewDomainError("IDENTITY_CANNOT_MODIFY_SELF", "cannot modify your own platform role or status")
	ErrLastSuperAdmin       = kernel.NewDomainError("IDENTITY_LAST_SUPER_ADMIN", "cannot remove the last super admin")
)
