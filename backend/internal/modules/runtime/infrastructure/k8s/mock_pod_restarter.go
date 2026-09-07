package k8s

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MockPodRestarter struct{}

func NewMockPodRestarter() *MockPodRestarter {
	return &MockPodRestarter{}
}

func (m *MockPodRestarter) Restart(
	context.Context,
	kernel.ProjectID,
	string,
	*model.Pod,
) error {
	return nil
}

var _ port.PodRestarter = (*MockPodRestarter)(nil)
