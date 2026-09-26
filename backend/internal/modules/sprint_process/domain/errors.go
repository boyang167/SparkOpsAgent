package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrProductNotFound                         = kernel.NewDomainError("PRODUCT_NOT_FOUND", "product not found")
	ErrWorkspaceSprintProcessTemplateNotFound  = kernel.NewDomainError("WORKSPACE_SPRINT_PROCESS_TEMPLATE_NOT_FOUND", "workspace sprint process template not found")
	ErrSprintProcessNotFound                   = kernel.NewDomainError("SPRINT_PROCESS_NOT_FOUND", "sprint process not found")
	ErrInvalidSprintProcess          = kernel.NewDomainError("SPRINT_PROCESS_INVALID", "invalid sprint process configuration")
	ErrSprintProcessInUse            = kernel.NewDomainError("SPRINT_PROCESS_IN_USE", "sprint process is referenced by sprints")
	ErrSprintProcessDefaultProtected = kernel.NewDomainError("SPRINT_PROCESS_DEFAULT_PROTECTED", "default sprint process cannot be deleted")
)
