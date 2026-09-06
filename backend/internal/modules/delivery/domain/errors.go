package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrBuildNotFound                 = kernel.NewDomainError("DELIVERY_BUILD_NOT_FOUND", "build not found")
	ErrProjectAccessDenied           = kernel.NewDomainError("DELIVERY_PROJECT_ACCESS_DENIED", "project access denied")
	ErrProjectNotActive              = kernel.NewDomainError("DELIVERY_PROJECT_NOT_ACTIVE", "project is not active")
	ErrBuildAlreadyRunning           = kernel.NewDomainError("DELIVERY_BUILD_RUNNING", "a build is already running for this project")
	ErrBuildNotRunning               = kernel.NewDomainError("DELIVERY_BUILD_NOT_RUNNING", "build is not running")
	ErrBuildNotRollbackable          = kernel.NewDomainError("DELIVERY_BUILD_NOT_ROLLBACKABLE", "only successful builds can be rolled back")
	ErrInvalidCronJob                = kernel.NewDomainError("DELIVERY_INVALID_CRONJOB", "invalid cronjob request")
	ErrInvalidDeploymentBuildRequest = kernel.NewDomainError("DELIVERY_INVALID_DEPLOYMENT_BUILD_REQUEST", "invalid deployment build request")
	ErrProjectJenkinsNotConfigured   = kernel.NewDomainError("DELIVERY_PROJECT_JENKINS_NOT_CONFIGURED", "project jenkins secret is not configured")
)
