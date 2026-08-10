package jenkins

import (
	"context"
	"fmt"
	"strings"
	"time"

	gjenkins "github.com/bndr/gojenkins"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Executor struct {
	client    *gjenkins.Jenkins
	jobSuffix string
}

func NewExecutor(cfg config.JenkinsConfig) (*Executor, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("jenkins url is required")
	}
	client := gjenkins.CreateJenkins(nil, cfg.URL, cfg.User, cfg.Token)
	if _, err := client.Init(context.Background()); err != nil {
		return nil, fmt.Errorf("jenkins init: %w", err)
	}
	suffix := cfg.JobSuffix
	if suffix == "" {
		suffix = "-build"
	}
	return &Executor{client: client, jobSuffix: suffix}, nil
}

func (e *Executor) RunStreaming(
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
	jobName := projectName + e.jobSuffix
	header := fmt.Sprintf("[jenkins] job=%s branch=%s build=%d\n", jobName, branch, buildNumber)
	if hub != nil {
		hub.Publish(buildID, header)
	}
	var buf strings.Builder
	buf.WriteString(header)

	job, err := e.client.GetJob(ctx, jobName)
	if err != nil {
		errLine := fmt.Sprintf("jenkins job not found: %s (%v)\n", jobName, err)
		buf.WriteString(errLine)
		if hub != nil {
			hub.Publish(buildID, errLine)
		}
		return buf.String(), err
	}

	params := map[string]string{"BRANCH": branch}
	if rollbackOf > 0 {
		params["ROLLBACK_OF"] = fmt.Sprintf("%d", rollbackOf)
	}
	queueID, err := job.InvokeSimple(ctx, params)
	if err != nil {
		return buf.String(), fmt.Errorf("invoke jenkins: %w", err)
	}
	build, err := e.client.GetBuildFromQueueID(ctx, job, queueID)
	if err != nil {
		return buf.String(), fmt.Errorf("wait build: %w", err)
	}

	for build.IsRunning(ctx) {
		select {
		case <-ctx.Done():
			return buf.String(), ctx.Err()
		default:
		}
		chunk := fmt.Sprintf("[jenkins] running #%d...\n", build.GetBuildNumber())
		buf.WriteString(chunk)
		if hub != nil {
			hub.Publish(buildID, chunk)
		}
		time.Sleep(time.Second)
		build, _ = job.GetBuild(ctx, build.GetBuildNumber())
	}

	result := fmt.Sprintf("[jenkins] finished result=%s\n", build.GetResult())
	buf.WriteString(result)
	if hub != nil {
		hub.Publish(buildID, result)
	}
	if build.GetResult() != "SUCCESS" {
		return buf.String(), fmt.Errorf("jenkins result: %s", build.GetResult())
	}
	return buf.String(), nil
}

var _ port.JenkinsExecutor = (*Executor)(nil)
