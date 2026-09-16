package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidProductName       = kernel.NewDomainError("PRODUCT_INVALID_NAME", "invalid product name")
	ErrProductNotFound          = kernel.NewDomainError("PRODUCT_NOT_FOUND", "product not found")
	ErrProductPlanNotFound      = kernel.NewDomainError("PRODUCT_PLAN_NOT_FOUND", "product plan not found")
	ErrProductRequirementNotFound = kernel.NewDomainError("PRODUCT_REQUIREMENT_NOT_FOUND", "product requirement not found")
	ErrInvalidRequirementTransition = kernel.NewDomainError("PRODUCT_INVALID_REQUIREMENT_TRANSITION", "invalid requirement status transition")
	ErrWorkspaceAccessDenied        = kernel.NewDomainError("PRODUCT_ACCESS_DENIED", "workspace access denied")
	ErrReleasePlanNotFound          = kernel.NewDomainError("RELEASE_PLAN_NOT_FOUND", "release plan not found")
	ErrReleasePlanJobNotFound       = kernel.NewDomainError("RELEASE_PLAN_JOB_NOT_FOUND", "release plan job not found")
	ErrInvalidReleasePlanTransition = kernel.NewDomainError("RELEASE_PLAN_INVALID_TRANSITION", "invalid release plan status transition")
	ErrInvalidReleasePlanJobTransition = kernel.NewDomainError("RELEASE_PLAN_JOB_INVALID_TRANSITION", "invalid release plan job transition")
	ErrEnvironmentNotFound             = kernel.NewDomainError("ENVIRONMENT_NOT_FOUND", "environment not found")
	ErrEnvironmentProjectNotFound      = kernel.NewDomainError("ENVIRONMENT_PROJECT_NOT_FOUND", "environment project not found")
	ErrInvalidEnvironmentInput           = kernel.NewDomainError("ENVIRONMENT_INVALID_INPUT", "invalid environment input")
	ErrEnvironmentClusterNotAuthorized   = kernel.NewDomainError("ENVIRONMENT_CLUSTER_NOT_AUTHORIZED", "cluster not authorized for workspace")
	ErrEnvironmentProductMismatch        = kernel.NewDomainError("ENVIRONMENT_PRODUCT_MISMATCH", "environment does not belong to product")
	ErrReleaseWindowClosed               = kernel.NewDomainError("RELEASE_WINDOW_CLOSED", "current time is outside release window")
	ErrReleasePlanJobNotReady            = kernel.NewDomainError("RELEASE_PLAN_JOB_NOT_READY", "release plan job is not ready for this action")
	ErrInvalidWorkflowRef                = kernel.NewDomainError("RELEASE_PLAN_INVALID_WORKFLOW_REF", "invalid workflow reference on release plan job")
)
