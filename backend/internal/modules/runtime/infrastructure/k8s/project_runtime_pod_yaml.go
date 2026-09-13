package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// PodYAML returns the live Kubernetes Pod as YAML (kubectl get -o yaml style).
func (i *ProjectRuntimeInspector) PodYAML(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	podID kernel.PodID,
) (string, error) {
	if i == nil || i.access == nil {
		return "", domain.ErrPodNotFound
	}
	session, err := i.access.Open(ctx, wsID, projectID)
	if err != nil {
		return "", err
	}
	pods, err := listProjectPods(ctx, session.Client, projectID, session.Namespace, session.ProjectName)
	if err != nil {
		return "", err
	}
	var name, namespace string
	for _, pod := range pods {
		if pod.ID == podID {
			name = pod.Name
			namespace = pod.Namespace
			break
		}
	}
	if name == "" {
		return "", domain.ErrPodNotFound
	}
	item, err := session.Client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", domain.ErrPodNotFound
		}
		return "", err
	}
	return marshalPodYAML(item)
}

func marshalPodYAML(pod *corev1.Pod) (string, error) {
	cleaned := pod.DeepCopy()
	cleaned.APIVersion = "v1"
	cleaned.Kind = "Pod"
	cleaned.ManagedFields = nil
	return marshalK8sYAML(cleaned, "v1", "Pod")
}
