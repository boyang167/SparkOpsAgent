package k8s

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

type MockPodLogs struct{}

func NewMockPodLogs() *MockPodLogs { return &MockPodLogs{} }

func (m *MockPodLogs) ReadLogs(_ context.Context, pod *model.Pod) (string, error) {
	return fmt.Sprintf(`2026-08-10T08:00:01Z INFO  Starting %s
2026-08-10T08:00:02Z INFO  Listening on :8080
2026-08-10T08:00:05Z INFO  Health check passed
2026-08-10T08:01:00Z INFO  Request GET /health 200 2ms
2026-08-10T08:02:15Z WARN  Slow query detected (mock)
`, pod.Container), nil
}

type MockPodTerminal struct{}

func NewMockPodTerminal() *MockPodTerminal { return &MockPodTerminal{} }

func (m *MockPodTerminal) StreamLines(_ context.Context, pod *model.Pod) []string {
	return []string{
		fmt.Sprintf("$ kubectl exec -it %s -n %s -- /bin/sh", pod.Name, pod.Namespace),
		"Defaulted container \"" + pod.Container + "\" out of: " + pod.Container,
		"Connecting to pod...",
		"# whoami",
		"sparkops",
		"# hostname",
		pod.Name,
		"# ps aux | head -n 3",
		"PID   USER     COMMAND",
		"  1   sparkops ./app",
		fmt.Sprintf("# echo mock terminal for %s", pod.Name),
		"mock terminal for " + pod.Name,
		"# ",
	}
}

var _ port.PodLogReader = (*MockPodLogs)(nil)
var _ port.PodTerminal = (*MockPodTerminal)(nil)
