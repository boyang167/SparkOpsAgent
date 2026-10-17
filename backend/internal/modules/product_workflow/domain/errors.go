package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrWorkflowTemplateNotFound          = kernel.NewDomainError("WORKFLOW_TEMPLATE_NOT_FOUND", "workflow template not found")
	ErrProductWorkflowNotFound           = kernel.NewDomainError("PRODUCT_WORKFLOW_NOT_FOUND", "product workflow not found")
	ErrProductWorkflowRunNotFound        = kernel.NewDomainError("PRODUCT_WORKFLOW_RUN_NOT_FOUND", "product workflow run not found")
	ErrProductWorkflowRunConflict        = kernel.NewDomainError("PRODUCT_WORKFLOW_RUN_CONFLICT", "product workflow run was updated concurrently")
	ErrWorkspaceAccessDenied             = kernel.NewDomainError("WORKFLOW_TEMPLATE_ACCESS_DENIED", "workspace access denied")
	ErrProductNotInWorkspace             = kernel.NewDomainError("PRODUCT_NOT_IN_WORKSPACE", "product not in workspace")
	ErrInvalidWorkflowInput              = kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "invalid workflow input")
	ErrWorkflowRunBusy                   = kernel.NewDomainError("PRODUCT_WORKFLOW_RUN_BUSY", "已有运行中的实例")
	ErrWorkflowHookRejected              = kernel.NewDomainError("PRODUCT_WORKFLOW_HOOK_REJECTED", "invalid workflow hook token")
	ErrApprovalNotPending                = kernel.NewDomainError("PRODUCT_WORKFLOW_APPROVAL_NOT_PENDING", "approval is not pending")
	ErrApprovalForbidden                 = kernel.NewDomainError("PRODUCT_WORKFLOW_APPROVAL_FORBIDDEN", "approval actor is not allowed")
	ErrStageNotWaitingManual             = kernel.NewDomainError("PRODUCT_WORKFLOW_STAGE_NOT_WAITING", "stage is not waiting for manual execution")
	ErrRunNotPausable                    = kernel.NewDomainError("PRODUCT_WORKFLOW_RUN_NOT_PAUSABLE", "workflow run cannot be paused")
	ErrRunNotPaused                      = kernel.NewDomainError("PRODUCT_WORKFLOW_RUN_NOT_PAUSED", "workflow run is not paused")
	ErrRunNotRetryable                   = kernel.NewDomainError("PRODUCT_WORKFLOW_RUN_NOT_RETRYABLE", "只有失败的执行记录可以重试")
	ErrRunDefinitionChanged              = kernel.NewDomainError("PRODUCT_WORKFLOW_RUN_DEFINITION_CHANGED", "工作流阶段或任务已修改，无法从失败处重试，请重新执行")
	ErrRegistryNotFound                  = kernel.NewDomainError("PRODUCT_WORKFLOW_REGISTRY_NOT_FOUND", "image registry secret not found")
	ErrWorkflowTemplateReadonly          = kernel.NewDomainError("WORKFLOW_TEMPLATE_READONLY", "system workflow template is readonly")
	ErrWorkflowTemplateKeyExists         = kernel.NewDomainError("WORKFLOW_TEMPLATE_KEY_EXISTS", "workflow template key already exists")
	ErrInvalidWorkflowTemplateTransition = kernel.NewDomainError("WORKFLOW_TEMPLATE_INVALID_TRANSITION", "invalid workflow template status transition")
)
