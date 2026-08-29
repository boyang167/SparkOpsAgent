package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrPodNotFound           = kernel.NewDomainError("K8S_POD_NOT_FOUND", "pod not found")
	ErrResourceNotFound      = kernel.NewDomainError("K8S_RESOURCE_NOT_FOUND", "resource not found")
	ErrProjectAccessDenied   = kernel.NewDomainError("K8S_PROJECT_ACCESS_DENIED", "project access denied")
	ErrKubeconfigMissing     = kernel.NewDomainError("K8S_KUBECONFIG_MISSING", "kubeconfig is required")
	ErrKubeconfigInvalid     = kernel.NewDomainError("K8S_KUBECONFIG_INVALID", "kubeconfig is invalid")
	ErrClusterServerMismatch = kernel.NewDomainError(
		"K8S_CLUSTER_SERVER_MISMATCH", "kubeconfig server does not match the target cluster",
	)
	ErrManifestInvalid = kernel.NewDomainError(
		"K8S_MANIFEST_INVALID", "kubernetes manifest is invalid",
	)
	ErrManifestUnsupported = kernel.NewDomainError(
		"K8S_MANIFEST_UNSUPPORTED", "kubernetes manifest kind is not supported",
	)
	ErrManifestNamespaceMismatch = kernel.NewDomainError(
		"K8S_MANIFEST_NAMESPACE_MISMATCH", "kubernetes manifest namespace does not match the target",
	)
	ErrManifestImageTagMissing = kernel.NewDomainError(
		"K8S_MANIFEST_IMAGE_TAG_MISSING", "deployment container image tag is required",
	)
	ErrManifestImageTagMismatch = kernel.NewDomainError(
		"K8S_MANIFEST_IMAGE_TAG_MISMATCH", "deployment does not contain the target image tag",
	)
	ErrDeploymentProgressDeadlineExceeded = kernel.NewDomainError(
		"K8S_DEPLOYMENT_PROGRESS_DEADLINE_EXCEEDED", "deployment exceeded its progress deadline",
	)
	ErrServiceDeleting = kernel.NewDomainError(
		"K8S_SERVICE_DELETING", "service is being deleted",
	)
)
