package k8s

import (
	"context"
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"gopkg.in/yaml.v3"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// DeploymentYAML returns the live Kubernetes Deployment as YAML (kubectl get -o yaml style).
func (i *ProjectRuntimeInspector) DeploymentYAML(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (string, error) {
	if i == nil || i.access == nil {
		return "", domain.ErrResourceNotFound
	}
	session, err := i.access.Open(ctx, wsID, projectID)
	if err != nil {
		return "", err
	}
	_, dep, err := loadDeploymentOverview(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", domain.ErrResourceNotFound
		}
		return "", err
	}
	if dep == nil {
		return "", domain.ErrResourceNotFound
	}
	return marshalDeploymentYAML(dep)
}

func marshalDeploymentYAML(dep *appsv1.Deployment) (string, error) {
	cleaned := dep.DeepCopy()
	cleaned.APIVersion = appsv1.SchemeGroupVersion.String()
	cleaned.Kind = "Deployment"
	cleaned.ManagedFields = nil
	return marshalK8sYAML(cleaned, "apps/v1", "Deployment")
}

func marshalK8sYAML(obj any, apiVersion, kind string) (string, error) {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return "", err
	}
	doc["apiVersion"] = apiVersion
	doc["kind"] = kind
	delete(doc, "managedFields")
	if metadata, ok := doc["metadata"].(map[string]any); ok {
		delete(metadata, "managedFields")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "generation")
		delete(metadata, "creationTimestamp")
	}
	yamlBytes, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(yamlBytes), nil
}

// MarshalK8sYAML exports a Kubernetes object as apply-friendly YAML.
func MarshalK8sYAML(obj any, apiVersion, kind string) (string, error) {
	return marshalK8sYAML(obj, apiVersion, kind)
}
