package jenkins

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MockExecutor struct{}

func NewMockExecutor() *MockExecutor { return &MockExecutor{} }

func (e *MockExecutor) RunStreaming(
	ctx context.Context,
	buildID kernel.BuildID,
	projectName, branch string,
	buildNumber int,
	rollbackOf int,
	hub port.LogPublisher,
) (string, error) {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		projectName = "app"
	}
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}

	lines := []string{
		fmt.Sprintf("[mock-jenkins] job=%s-build build=%d branch=%s", projectName, buildNumber, branch),
	}
	if rollbackOf > 0 {
		lines = append(lines, fmt.Sprintf("[rollback] redeploying artifact from build #%d", rollbackOf))
	}
	lines = append(lines,
		"Checking out source...",
		"mvn clean package -DskipTests",
		fmt.Sprintf("docker build -t %s:%d .", projectName, buildNumber),
		"BUILD SUCCESS",
	)

	var buf strings.Builder
	for _, line := range lines {
		select {
		case <-ctx.Done():
			return buf.String(), ctx.Err()
		default:
		}
		chunk := line + "\n"
		buf.WriteString(chunk)
		if hub != nil {
			hub.Publish(buildID, chunk)
		}
		time.Sleep(350 * time.Millisecond)
	}
	return buf.String(), nil
}

var _ port.JenkinsExecutor = (*MockExecutor)(nil)
