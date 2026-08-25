package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidProjectName           = kernel.NewDomainError("WORKSPACE_APP_INVALID_NAME", "invalid project name")
	ErrProjectNotFound              = kernel.NewDomainError("WORKSPACE_APP_PROJECT_NOT_FOUND", "project not found")
	ErrProjectNameExists            = kernel.NewDomainError("WORKSPACE_APP_PROJECT_EXISTS", "project name already exists")
	ErrProjectNotActive             = kernel.NewDomainError("WORKSPACE_APP_PROJECT_NOT_ACTIVE", "project is not active")
	ErrProjectAlreadyArchived       = kernel.NewDomainError("WORKSPACE_APP_PROJECT_ARCHIVED", "project already archived")
	ErrWorkspaceAccessDenied        = kernel.NewDomainError("WORKSPACE_APP_ACCESS_DENIED", "workspace access denied")
	ErrProjectMemberNotFound        = kernel.NewDomainError("WORKSPACE_APP_PROJECT_MEMBER_NOT_FOUND", "project member not found")
	ErrProjectAccessDenied          = kernel.NewDomainError("PROJECT_ACCESS_DENIED", "project access denied")
	ErrProjectConfigIncomplete      = kernel.NewDomainError("PROJECT_CONFIG_INCOMPLETE", "project configuration is incomplete")
	ErrProjectReviewInvalidState    = kernel.NewDomainError("PROJECT_REVIEW_INVALID_STATE", "project review state does not allow this operation")
	ErrProjectReviewLocked          = kernel.NewDomainError("PROJECT_REVIEW_LOCKED", "project configuration is locked during review")
	ErrProjectReviewSelf            = kernel.NewDomainError("PROJECT_REVIEW_SELF", "submitter cannot review own project")
	ErrProjectReviewCommentRequired = kernel.NewDomainError("INVALID_REQUEST", "rejection comment is required")
	ErrProjectReviewForbidden       = kernel.NewDomainError("AUTH_FORBIDDEN", "project review permission required")
	ErrInvalidTemplateName          = kernel.NewDomainError("WORKSPACE_APP_INVALID_TEMPLATE_NAME", "invalid template name")
	ErrTemplateNotFound             = kernel.NewDomainError("WORKSPACE_APP_TEMPLATE_NOT_FOUND", "template not found")
	ErrInvalidSecretName            = kernel.NewDomainError("WORKSPACE_APP_INVALID_SECRET_NAME", "invalid secret name")
	ErrSecretNotFound               = kernel.NewDomainError("WORKSPACE_APP_SECRET_NOT_FOUND", "secret not found")
)
