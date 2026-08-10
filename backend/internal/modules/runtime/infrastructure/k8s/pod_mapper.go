package k8s

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func DefaultNamespace(projectName string) string {
	name := strings.TrimSpace(strings.ToLower(projectName))
	if name == "" {
		return "sparkops-demo"
	}
	return "sparkops-" + name
}

func PodIDFor(projectID kernel.ProjectID, namespace, podName string) kernel.PodID {
	return kernel.PodID{UUID: uuid.NewSHA1(projectID.UUID, []byte(namespace+"/"+podName))}
}

func MapPod(projectID kernel.ProjectID, item corev1.Pod) *model.Pod {
	containerName, image, restarts, startedAt := primaryContainerStatus(item)
	return &model.Pod{
		ID:        PodIDFor(projectID, item.Namespace, item.Name),
		ProjectID: projectID,
		Name:      item.Name,
		Namespace: item.Namespace,
		Phase:     model.PodPhase(item.Status.Phase),
		Node:      item.Spec.NodeName,
		Restarts:  restarts,
		Container: containerName,
		Image:     image,
		StartedAt: startedAt,
	}
}

func primaryContainerStatus(pod corev1.Pod) (name, image string, restarts int, startedAt string) {
	if len(pod.Spec.Containers) > 0 {
		name = pod.Spec.Containers[0].Name
		image = pod.Spec.Containers[0].Image
	}
	for _, st := range pod.Status.ContainerStatuses {
		if name != "" && st.Name != name {
			continue
		}
		if name == "" {
			name = st.Name
			image = st.Image
		}
		restarts = int(st.RestartCount)
		if st.State.Running != nil {
			startedAt = st.State.Running.StartedAt.UTC().Format(time.RFC3339)
		}
		break
	}
	if startedAt == "" && !pod.CreationTimestamp.IsZero() {
		startedAt = pod.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	return name, image, restarts, startedAt
}

func ValidatePod(pod *model.Pod) error {
	if pod == nil {
		return fmt.Errorf("pod is nil")
	}
	if pod.Name == "" || pod.Namespace == "" {
		return fmt.Errorf("pod name and namespace are required")
	}
	if pod.Container == "" {
		return fmt.Errorf("pod container is required")
	}
	return nil
}
