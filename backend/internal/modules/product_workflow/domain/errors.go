package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrWorkflowTemplateNotFound = kernel.NewDomainError("WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found")
	ErrProductWorkflowNotFound  = kernel.NewDomainError("PRODUCT_WORKFLOW_NOT_FOUND", "product workflow not found")
	ErrProductWorkflowRunNotFound = kernel.NewDomainError("PRODUCT_WORKFLOW_RUN_NOT_FOUND", "product workflow run not found")
	ErrWorkspaceAccessDenied    = kernel.NewDomainError("WORKFLOW_TEMPLATE_ACCESS_DENIED", "workspace access denied")
	ErrProductNotInWorkspace    = kernel.NewDomainError("PRODUCT_NOT_IN_WORKSPACE", "product not in workspace")
	ErrInvalidWorkflowInput     = kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "invalid workflow input")
	ErrApprovalNotPending                = kernel.NewDomainError("PRODUCT_WORKFLOW_APPROVAL_NOT_PENDING", "approval is not pending")
	ErrStageNotWaitingManual             = kernel.NewDomainError("PRODUCT_WORKFLOW_STAGE_NOT_WAITING", "stage is not waiting for manual execution")
	ErrRegistryNotFound                  = kernel.NewDomainError("PRODUCT_WORKFLOW_REGISTRY_NOT_FOUND", "image registry secret not found")
	ErrWorkflowTemplateReadonly          = kernel.NewDomainError("WORKFLOW_TEMPLATE_READONLY", "system workflow template is readonly")
	ErrWorkflowTemplateKeyExists         = kernel.NewDomainError("WORKFLOW_TEMPLATE_KEY_EXISTS", "workflow template key already exists")
	ErrInvalidWorkflowTemplateTransition = kernel.NewDomainError("WORKFLOW_TEMPLATE_INVALID_TRANSITION", "invalid workflow template status transition")
)
