package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidProjectName      = kernel.NewDomainError("WORKSPACE_APP_INVALID_NAME", "invalid project name")
	ErrProjectNotFound         = kernel.NewDomainError("WORKSPACE_APP_PROJECT_NOT_FOUND", "project not found")
	ErrProjectNameExists       = kernel.NewDomainError("WORKSPACE_APP_PROJECT_EXISTS", "project name already exists")
	ErrProjectNotActive        = kernel.NewDomainError("WORKSPACE_APP_PROJECT_NOT_ACTIVE", "project is not active")
	ErrProjectAlreadyArchived  = kernel.NewDomainError("WORKSPACE_APP_PROJECT_ARCHIVED", "project already archived")
	ErrWorkspaceAccessDenied   = kernel.NewDomainError("WORKSPACE_APP_ACCESS_DENIED", "workspace access denied")
	ErrProjectMemberNotFound   = kernel.NewDomainError("WORKSPACE_APP_PROJECT_MEMBER_NOT_FOUND", "project member not found")
	ErrProjectAccessDenied     = kernel.NewDomainError("PROJECT_ACCESS_DENIED", "project access denied")
	ErrInvalidTemplateName     = kernel.NewDomainError("WORKSPACE_APP_INVALID_TEMPLATE_NAME", "invalid template name")
	ErrInvalidDockerfileName   = kernel.NewDomainError("WORKSPACE_APP_INVALID_DOCKERFILE_NAME", "invalid dockerfile name")
)
