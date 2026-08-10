package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrPodNotFound         = kernel.NewDomainError("K8S_POD_NOT_FOUND", "pod not found")
	ErrResourceNotFound    = kernel.NewDomainError("K8S_RESOURCE_NOT_FOUND", "resource not found")
	ErrProjectAccessDenied = kernel.NewDomainError("K8S_PROJECT_ACCESS_DENIED", "project access denied")
)
