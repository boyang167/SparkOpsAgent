package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TriggerBuildHandler struct {
	builds       repository.BuildRepository
	checker      port.ProjectChecker
	jenkins      port.JenkinsExecutorProvider
	bus          eventbus.Bus
	hub          *stream.Hub
	registry     *runner.Registry
	onAsyncError func(error)
	workerCtx    context.Context
}

func NewTriggerBuildHandler(
	builds repository.BuildRepository,
	checker port.ProjectChecker,
	jenkins port.JenkinsExecutorProvider,
	bus eventbus.Bus,
	hub *stream.Hub,
	registry *runner.Registry,
	workerCtx context.Context,
	reporters ...func(error),
) *TriggerBuildHandler {
	handler := &TriggerBuildHandler{
		builds: builds, checker: checker, jenkins: jenkins,
		bus: bus, hub: hub, registry: registry,
		workerCtx: workerCtx,
	}
	if len(reporters) > 0 {
		handler.onAsyncError = reporters[0]
	}
	return handler
}

func (h *TriggerBuildHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	branch string,
) (*model.Build, error) {
	return h.trigger(ctx, wsID, memberID, projectID, port.BuildRequest{GitRefValue: branch}, false, nil)
}

func (h *TriggerBuildHandler) TriggerAutomated(
	ctx context.Context,
	projectID kernel.ProjectID,
	branch string,
) (*model.Build, error) {
	return h.TriggerAutomatedBuild(ctx, projectID, branch, "")
}

func (h *TriggerBuildHandler) TriggerAutomatedBuild(
	ctx context.Context,
	projectID kernel.ProjectID,
	branch string,
	imageRepository string,
) (*model.Build, error) {
	auto, err := h.checker.ProjectAutomationContext(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return h.trigger(ctx, auto.WorkspaceID, auto.MemberID, projectID, port.BuildRequest{
		GitRefValue:     branch,
		ImageRepository: strings.TrimSpace(imageRepository),
	}, false, nil)
}

func (h *TriggerBuildHandler) TriggerDeployment(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	req port.BuildRequest,
) (*model.Build, error) {
	return h.TriggerDeploymentAssociated(ctx, wsID, memberID, projectID, req, nil)
}

type BeforeBuildStart = func(context.Context, *model.Build) error

func (h *TriggerBuildHandler) TriggerDeploymentAssociated(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	req port.BuildRequest,
	beforeStart BeforeBuildStart,
) (*model.Build, error) {
	var err error
	req, err = normalizeDeploymentRequest(req)
	if err != nil {
		return nil, err
	}
	return h.trigger(ctx, wsID, memberID, projectID, req, true, beforeStart)
}

func normalizeDeploymentRequest(req port.BuildRequest) (port.BuildRequest, error) {
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.ProjectName = strings.TrimSpace(req.ProjectName)
	req.Folder = strings.TrimSpace(req.Folder)
	req.Job = strings.TrimSpace(req.Job)
	req.GitRefValue = strings.TrimSpace(req.GitRefValue)
	req.ImageTag = strings.TrimSpace(req.ImageTag)
	req.ImageRepository = strings.TrimSpace(req.ImageRepository)
	if req.Namespace == "" || req.ProjectName == "" || req.Folder == "" || req.Job == "" ||
		req.GitRefValue == "" || req.ImageTag == "" || req.ImageRepository == "" ||
		req.Folder != req.Namespace || req.Job != req.ProjectName {
		return port.BuildRequest{}, domain.ErrInvalidDeploymentBuildRequest
	}
	return req, nil
}

func (h *TriggerBuildHandler) trigger(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	req port.BuildRequest,
	deployment bool,
	beforeStart BeforeBuildStart,
) (*model.Build, error) {
	projectName, err := h.validateProject(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	if deployment && req.ProjectName != projectName {
		return nil, domain.ErrInvalidDeploymentBuildRequest
	}
	if deployment {
		projectNamespace, err := h.checker.ProjectNamespace(ctx, projectID)
		if err != nil {
			return nil, err
		}
		if req.Namespace != strings.TrimSpace(projectNamespace) {
			return nil, domain.ErrInvalidDeploymentBuildRequest
		}
	}
	if req.Job == "" {
		req.Job = projectName
		if !deployment {
			req.Job += "-build"
		}
	}

	buildNumber, err := h.builds.NextBuildNumber(ctx, projectID)
	if err != nil {
		return nil, err
	}

	build := model.NewBuild(projectID, memberID, wsID, req.GitRefValue)
	build.BuildNumber = buildNumber
	build.JenkinsJob = req.Job
	if req.Folder != "" {
		build.JenkinsJob = fmt.Sprintf("%s/%s", req.Folder, req.Job)
	}
	if err := h.builds.Save(ctx, build); err != nil {
		return nil, err
	}
	if beforeStart != nil {
		if err := beforeStart(ctx, build); err != nil {
			build.Fail("deployment association failed", "")
			if updateErr := h.builds.Update(ctx, build); updateErr != nil {
				return nil, fmt.Errorf("finish unassociated build: %w", updateErr)
			}
			return nil, err
		}
	}
	executionStartedAt := time.Now().UTC()
	build.ExecutionStartedAt = &executionStartedAt
	if err := h.builds.Update(ctx, build); err != nil {
		return nil, fmt.Errorf("mark build execution started: %w", err)
	}

	go h.executeBuild(build, projectName, req, memberID, wsID, projectID)
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
	req port.BuildRequest,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) {
	execCtx, cancel := context.WithCancel(h.workerCtx)
	h.registry.Register(build.ID.String(), cancel)
	defer h.registry.Done(build.ID.String())

	jenkins, err := h.jenkins.ForProject(execCtx, wsID, projectID)
	if err != nil {
		build.Fail(err.Error(), "")
		if updateErr := h.updateBuildWithRetry(h.workerCtx, build); updateErr != nil {
			h.reportAsyncError(fmt.Errorf("persist jenkins config failure: %w", updateErr))
		}
		h.hub.Complete(build.ID, build.ConsoleLog)
		return
	}

	consoleLog, execErr := jenkins.RunStreaming(
		execCtx, build.ID, req, h.hub,
	)

	if execErr != nil {
		if execCtx.Err() != nil {
			build.Abort(consoleLog)
		} else {
			build.Fail(execErr.Error(), consoleLog)
		}
	} else {
		build.Succeed(build.BuildNumber, build.JenkinsJob, consoleLog)
	}

	if err := h.updateBuildWithRetry(h.workerCtx, build); err != nil {
		h.hub.Complete(build.ID, build.ConsoleLog)
		h.reportAsyncError(fmt.Errorf("persist completed build: %w", err))
		return
	}
	h.hub.Complete(build.ID, build.ConsoleLog)
	if err := publishBuildCompleted(h.workerCtx, h.bus, event.NewBuildCompleted(
		build.ID, projectID, memberID, wsID, projectName, string(build.Status), build.BuildNumber,
	)); err != nil {
		h.reportAsyncError(fmt.Errorf("publish completed build: %w", err))
	}
}

func (h *TriggerBuildHandler) updateBuildWithRetry(ctx context.Context, build *model.Build) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = h.builds.Update(ctx, build); err == nil {
			return nil
		}
		if attempt == 2 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

func (h *TriggerBuildHandler) reportAsyncError(err error) {
	if h.onAsyncError != nil {
		h.onAsyncError(err)
	}
}

func publishBuildCompleted(ctx context.Context, bus eventbus.Bus, completed events.Event) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = bus.Publish(ctx, completed); err == nil {
			return nil
		}
		if attempt == 2 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}
