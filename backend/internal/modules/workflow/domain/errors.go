package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidWorkflowName    = kernel.NewDomainError("WORKFLOW_INVALID_NAME", "invalid workflow name")
	ErrWorkflowNotFound       = kernel.NewDomainError("WORKFLOW_NOT_FOUND", "workflow not found")
	ErrWorkflowNameExists     = kernel.NewDomainError("WORKFLOW_NAME_EXISTS", "workflow name already exists")
	ErrWorkflowArchived       = kernel.NewDomainError("WORKFLOW_ARCHIVED", "workflow is archived")
	ErrWorkflowNotActive      = kernel.NewDomainError("WORKFLOW_NOT_ACTIVE", "workflow is not active")
	ErrWorkflowRunNotFound    = kernel.NewDomainError("WORKFLOW_RUN_NOT_FOUND", "workflow run not found")
	ErrProjectAccessDenied    = kernel.NewDomainError("WORKFLOW_PROJECT_ACCESS_DENIED", "project access denied")
	ErrProjectNotActive       = kernel.NewDomainError("WORKFLOW_PROJECT_NOT_ACTIVE", "project is not active")
)
