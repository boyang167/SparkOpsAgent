package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrRoleNotFound          = kernel.NewDomainError("GOVERNANCE_ROLE_NOT_FOUND", "role not found")
	ErrBindingNotFound       = kernel.NewDomainError("GOVERNANCE_BINDING_NOT_FOUND", "role binding not found")
	ErrPermissionDenied      = kernel.NewDomainError("GOVERNANCE_PERMISSION_DENIED", "permission denied")
	ErrRoleAlreadyExists     = kernel.NewDomainError("GOVERNANCE_ROLE_EXISTS", "role already exists")
	ErrMemberNotFound        = kernel.NewDomainError("GOVERNANCE_MEMBER_NOT_FOUND", "member not found")
	ErrMemberAlreadyInWorkspace = kernel.NewDomainError("GOVERNANCE_MEMBER_EXISTS", "member already in workspace")
	ErrRoleProtected         = kernel.NewDomainError("GOVERNANCE_ROLE_PROTECTED", "protected role cannot be deleted")
	ErrPermissionsRequired   = kernel.NewDomainError("GOVERNANCE_PERMISSIONS_REQUIRED", "at least one permission required")
	ErrRoleHasChildren       = kernel.NewDomainError("GOVERNANCE_ROLE_HAS_CHILDREN", "role has child roles")
	ErrInvalidParentRole     = kernel.NewDomainError("GOVERNANCE_INVALID_PARENT", "invalid parent role")
	ErrInvalidRoleUpdate     = kernel.NewDomainError("GOVERNANCE_INVALID_ROLE_UPDATE", "no fields to update")
	ErrMenuNotFound          = kernel.NewDomainError("GOVERNANCE_MENU_NOT_FOUND", "menu not found")
	ErrMenuHasChildren       = kernel.NewDomainError("GOVERNANCE_MENU_HAS_CHILDREN", "menu has child menus")
)
