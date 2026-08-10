package command

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TriggerBuildHandler struct {
	builds   repository.BuildRepository
	checker  port.ProjectChecker
	jenkins  port.JenkinsExecutor
	bus      eventbus.Bus
	hub      *stream.Hub
	registry *runner.Registry
}

func NewTriggerBuildHandler(
	builds repository.BuildRepository,
	checker port.ProjectChecker,
	jenkins port.JenkinsExecutor,
	bus eventbus.Bus,
	hub *stream.Hub,
	registry *runner.Registry,
) *TriggerBuildHandler {
	return &TriggerBuildHandler{
		builds: builds, checker: checker, jenkins: jenkins,
		bus: bus, hub: hub, registry: registry,
	}
}

func (h *TriggerBuildHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	branch string,
) (*model.Build, error) {
	return h.trigger(ctx, wsID, memberID, projectID, branch)
}

func (h *TriggerBuildHandler) TriggerAutomated(
	ctx context.Context,
	projectID kernel.ProjectID,
	branch string,
) (*model.Build, error) {
	auto, err := h.checker.ProjectAutomationContext(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return h.trigger(ctx, auto.WorkspaceID, auto.MemberID, projectID, branch)
}

func (h *TriggerBuildHandler) trigger(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	branch string,
) (*model.Build, error) {
	projectName, err := h.validateProject(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}

	buildNumber, err := h.builds.NextBuildNumber(ctx, projectID)
	if err != nil {
		return nil, err
	}

	build := model.NewBuild(projectID, memberID, wsID, branch)
	build.BuildNumber = buildNumber
	build.JenkinsJob = fmt.Sprintf("%s-build", projectName)
	if err := h.builds.Save(ctx, build); err != nil {
		return nil, err
	}

	go h.executeBuild(build, projectName, 0, memberID, wsID, projectID)
	return build, nil
}

func (h *TriggerBuildHandler) validateProject(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (string, error) {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return "", err
	}
	if !inWs {
		return "", domain.ErrProjectAccessDenied
	}
	active, err := h.checker.ProjectIsActive(ctx, projectID)
	if err != nil {
		return "", err
	}
	if !active {
		return "", domain.ErrProjectNotActive
	}
	running, err := h.builds.HasRunningBuild(ctx, projectID)
	if err != nil {
		return "", err
	}
	if running {
		return "", domain.ErrBuildAlreadyRunning
	}
	return h.checker.ProjectName(ctx, projectID)
}

func (h *TriggerBuildHandler) executeBuild(
	build *model.Build,
	projectName string,
	rollbackOf int,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) {
	ctx, cancel := context.WithCancel(context.Background())
	h.registry.Register(build.ID.String(), cancel)
	defer h.registry.Done(build.ID.String())

	consoleLog, execErr := h.jenkins.RunStreaming(
		ctx, build.ID, projectName, build.Branch, build.BuildNumber, rollbackOf, h.hub,
	)

	if execErr != nil {
		if ctx.Err() != nil {
			build.Abort(consoleLog)
		} else {
			build.Fail(execErr.Error(), consoleLog)
		}
	} else {
		build.Succeed(build.BuildNumber, build.JenkinsJob, consoleLog)
	}

	_ = h.builds.Update(context.Background(), build)
	h.hub.Complete(build.ID, build.ConsoleLog)
	_ = h.bus.Publish(context.Background(), event.NewBuildCompleted(
		build.ID, projectID, memberID, wsID, projectName, string(build.Status), build.BuildNumber,
	))
}
