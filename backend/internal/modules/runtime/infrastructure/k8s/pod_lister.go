package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PodLister struct {
	client  kubernetes.Interface
	cluster string
	checker port.ProjectChecker
}

func NewPodLister(
	factory *platformk8s.Factory,
	cluster string,
	checker port.ProjectChecker,
) (*PodLister, error) {
	if factory == nil {
		return nil, fmt.Errorf("k8s factory is required")
	}
	client, err := factory.Get(cluster)
	if err != nil {
		return nil, err
	}
	return &PodLister{client: client, cluster: cluster, checker: checker}, nil
}

func NewPodListerWithClient(client kubernetes.Interface, checker port.ProjectChecker) *PodLister {
	return &PodLister{client: client, checker: checker}
}

func (l *PodLister) ListByProject(
	ctx context.Context,
	projectID kernel.ProjectID,
	projectName string,
) ([]*model.Pod, error) {
	namespace, err := l.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	items, err := l.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods in %s: %w", namespace, err)
	}
	out := make([]*model.Pod, 0, len(items.Items))
	for i := range items.Items {
		out = append(out, MapPod(projectID, items.Items[i]))
	}
	return out, nil
}

func (l *PodLister) FindByID(
	ctx context.Context,
	projectID kernel.ProjectID,
	projectName string,
	podID kernel.PodID,
) (*model.Pod, error) {
	pods, err := l.ListByProject(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	for _, pod := range pods {
		if pod.ID == podID {
			return pod, nil
		}
	}
	return nil, domain.ErrPodNotFound
}

var _ port.PodLister = (*PodLister)(nil)
