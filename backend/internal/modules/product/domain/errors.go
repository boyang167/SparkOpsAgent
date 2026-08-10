package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidProductName       = kernel.NewDomainError("PRODUCT_INVALID_NAME", "invalid product name")
	ErrProductNotFound          = kernel.NewDomainError("PRODUCT_NOT_FOUND", "product not found")
	ErrProductPlanNotFound      = kernel.NewDomainError("PRODUCT_PLAN_NOT_FOUND", "product plan not found")
	ErrProductRequirementNotFound = kernel.NewDomainError("PRODUCT_REQUIREMENT_NOT_FOUND", "product requirement not found")
	ErrInvalidRequirementTransition = kernel.NewDomainError("PRODUCT_INVALID_REQUIREMENT_TRANSITION", "invalid requirement status transition")
	ErrWorkspaceAccessDenied    = kernel.NewDomainError("PRODUCT_ACCESS_DENIED", "workspace access denied")
)
