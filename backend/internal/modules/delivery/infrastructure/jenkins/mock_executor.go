package jenkins

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MockExecutor struct {
	LastJobSpec      port.JenkinsJobSpec
	LastBuildRequest port.BuildRequest
	LastDisabledJob  string
}

func NewMockExecutor() *MockExecutor { return &MockExecutor{} }

func (e *MockExecutor) CreateOrUpdate(_ context.Context, spec port.JenkinsJobSpec) error {
	e.LastJobSpec = spec
	return nil
}

func (e *MockExecutor) SyncCredentials(_ context.Context, folder string, credentials []port.JenkinsCredential) error {
	if e.LastJobSpec.Folder == "" {
		e.LastJobSpec.Folder = folder
	}
	e.LastJobSpec.Credentials = credentials
	return nil
}

func (e *MockExecutor) RunStreaming(
	ctx context.Context,
	buildID kernel.BuildID,
	req port.BuildRequest,
	hub port.LogPublisher,
) (string, error) {
	if len(req.Credentials) > 0 && strings.TrimSpace(req.Folder) != "" {
		if err := e.SyncCredentials(ctx, req.Folder, req.Credentials); err != nil {
			return "", err
		}
	}
	e.LastBuildRequest = req
	req.Job = strings.TrimSpace(req.Job)
	if req.Job == "" {
		req.Job = "app-build"
	}
	if strings.TrimSpace(req.GitRefValue) == "" {
		req.GitRefValue = "main"
	}

	lines := []string{
		fmt.Sprintf("[mock-jenkins] job=%s branch=%s", req.Job, req.GitRefValue),
	}
	if req.RollbackOf > 0 {
		lines = append(lines, fmt.Sprintf("[rollback] redeploying artifact from build #%d", req.RollbackOf))
	}
	lines = append(lines,
		"Checking out source...",
		"mvn clean package -DskipTests",
		fmt.Sprintf("docker build -t %s .", req.Job),
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

func (e *MockExecutor) DisableJob(_ context.Context, folder, job string) error {
	e.LastDisabledJob = folder + "/" + job
	return nil
}

func (e *MockExecutor) IsJobDisabled(_ context.Context, folder, job string) (bool, error) {
	return e.LastDisabledJob == folder+"/"+job, nil
}

var _ port.JenkinsExecutor = (*MockExecutor)(nil)
var _ port.JenkinsJobDisabler = (*MockExecutor)(nil)
