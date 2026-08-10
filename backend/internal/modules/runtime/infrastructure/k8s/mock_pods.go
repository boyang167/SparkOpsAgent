package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MockPodLister struct{}

func NewMockPodLister() *MockPodLister { return &MockPodLister{} }

func (m *MockPodLister) ListByProject(_ context.Context, projectID kernel.ProjectID, projectName string) ([]*model.Pod, error) {
	projectName = sanitizeName(projectName)
	return []*model.Pod{
		m.makePod(projectID, projectName, "api-0", "Running", 0),
		m.makePod(projectID, projectName, "worker-0", "Running", 1),
	}, nil
}

func (m *MockPodLister) FindByID(ctx context.Context, projectID kernel.ProjectID, projectName string, podID kernel.PodID) (*model.Pod, error) {
	pods, err := m.ListByProject(ctx, projectID, projectName)
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

func (m *MockPodLister) makePod(
	projectID kernel.ProjectID,
	projectName, suffix, phase string,
	restarts int,
) *model.Pod {
	container := projectName + "-" + strings.TrimSuffix(suffix, "-0")
	if strings.HasSuffix(suffix, "worker-0") {
		container = projectName + "-worker"
	} else {
		container = projectName + "-api"
	}
	id := kernel.PodID{UUID: uuid.NewSHA1(projectID.UUID, []byte("pod:" + suffix))}
	return &model.Pod{
		ID:        id,
		ProjectID: projectID,
		Name:      fmt.Sprintf("%s-%s", projectName, suffix),
		Namespace: "sparkops-" + projectName,
		Phase:     model.PodPhase(phase),
		Node:      "mock-node-1",
		Restarts:  restarts,
		Container: container,
		Image:     fmt.Sprintf("sparkops/%s:latest", container),
		StartedAt: "2026-08-10T08:00:00Z",
	}
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "demo"
	}
	return name
}

var _ port.PodLister = (*MockPodLister)(nil)
