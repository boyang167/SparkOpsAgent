package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PlatformPodRestarter struct {
	client kubernetes.Interface
}

func NewPlatformPodRestarter(client kubernetes.Interface) *PlatformPodRestarter {
	return &PlatformPodRestarter{client: client}
}

func (r *PlatformPodRestarter) Restart(
	ctx context.Context,
	_ kernel.ProjectID,
	_ string,
	pod *model.Pod,
) error {
	if pod == nil {
		return fmt.Errorf("pod is required")
	}
	err := r.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete pod %q: %w", pod.Name, err)
	}
	return nil
}

var _ port.PodRestarter = (*PlatformPodRestarter)(nil)
