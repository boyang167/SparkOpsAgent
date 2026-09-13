package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidDeploymentTransition    = kernel.NewDomainError("DELIVERY_DEPLOYMENT_INVALID_TRANSITION", "invalid deployment run transition")
	ErrDeploymentRunNotFound          = kernel.NewDomainError("DELIVERY_DEPLOYMENT_NOT_FOUND", "deployment run not found")
	ErrDeploymentAlreadyRunning       = kernel.NewDomainError("DELIVERY_DEPLOYMENT_RUNNING", "a deployment is already running for this project")
	ErrProjectNotApproved             = kernel.NewDomainError("DELIVERY_PROJECT_NOT_APPROVED", "project is not approved")
	ErrDeploymentBindingMissing       = kernel.NewDomainError("DELIVERY_DEPLOYMENT_BINDING_MISSING", "required deployment binding is missing")
	ErrDeploymentClusterNotFound      = kernel.NewDomainError("DELIVERY_DEPLOYMENT_CLUSTER_NOT_FOUND", "cluster not found")
	ErrDeploymentClusterNotAuthorized = kernel.NewDomainError("DELIVERY_DEPLOYMENT_CLUSTER_NOT_AUTHORIZED", "cluster is not authorized for this workspace")
	ErrKubernetesKubeconfigMissing    = kernel.NewDomainError("DELIVERY_KUBECONFIG_MISSING", "kubernetes secret extra_config kubeconfig is required")
	ErrDeploymentPipelineEmpty        = kernel.NewDomainError("DELIVERY_DEPLOYMENT_PIPELINE_EMPTY", "rendered deployment pipeline is empty")
	ErrDeploymentRunConcurrentUpdate  = kernel.NewDomainError("DELIVERY_DEPLOYMENT_CONCURRENT_UPDATE", "deployment run was updated concurrently")
	ErrDeploymentBindingInvalid       = kernel.NewDomainError("DELIVERY_DEPLOYMENT_BINDING_INVALID", "deployment binding is invalid")
	ErrDeploymentBuildStalled         = kernel.NewDomainError("DELIVERY_DEPLOYMENT_BUILD_STALLED", "deployment build did not start or timed out")
	ErrDeploymentNotRollbackable      = kernel.NewDomainError("DELIVERY_DEPLOYMENT_NOT_ROLLBACKABLE", "no rollback target is available")
	ErrCrossClusterSyncTargetsMissing = kernel.NewDomainError("DELIVERY_CROSS_CLUSTER_TARGETS_MISSING", "no sync clusters configured for cross-cluster publish")
	ErrDeploymentNotSyncable            = kernel.NewDomainError("DELIVERY_DEPLOYMENT_NOT_SYNCABLE", "no succeeded deployment with image tag is available for sync")
	ErrDeploymentSyncManifestsEmpty     = kernel.NewDomainError("DELIVERY_DEPLOYMENT_SYNC_MANIFESTS_EMPTY", "at least one sync manifest is required")
	ErrDeploymentSyncManifestInvalid    = kernel.NewDomainError("DELIVERY_DEPLOYMENT_SYNC_MANIFEST_INVALID", "sync manifest is invalid")
)
