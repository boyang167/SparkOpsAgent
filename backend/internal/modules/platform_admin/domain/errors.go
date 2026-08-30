package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrSysConfigNotFound     = kernel.NewDomainError("PLATFORM_ADMIN_SYSCONFIG_NOT_FOUND", "system config not found")
	ErrClusterNotFound       = kernel.NewDomainError("PLATFORM_ADMIN_CLUSTER_NOT_FOUND", "cluster not found")
	ErrClusterNameRequired   = kernel.NewDomainError("PLATFORM_ADMIN_CLUSTER_NAME_REQUIRED", "cluster name is required")
	ErrProclaimNotFound      = kernel.NewDomainError("PLATFORM_ADMIN_PROCLAIM_NOT_FOUND", "proclaim not found")
	ErrWorkspaceAccessDenied = kernel.NewDomainError("PLATFORM_ADMIN_ACCESS_DENIED", "workspace access denied")
)
