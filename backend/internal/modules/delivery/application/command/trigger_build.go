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
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
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
	tx           postgres.Transactor
}

func NewTriggerBuildHandler(
	builds repository.BuildRepository,
	checker port.ProjectChecker,
	jenkins port.JenkinsExecutorProvider,
	bus eventbus.Bus,
	hub *stream.Hub,
	registry *runner.Registry,
	workerCtx context.Context,
	tx postgres.Transactor,
	reporters ...func(error),
) *TriggerBuildHandler {
	handler := &TriggerBuildHandler{
		builds: builds, checker: checker, jenkins: jenkins,
		bus: bus, hub: hub, registry: registry,
		workerCtx: workerCtx, tx: tx,
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
	return h.TriggerAutomatedBuild(ctx, projectID, branch, "", "", "", nil)
}

func (h *TriggerBuildHandler) TriggerAutomatedBuild(
	ctx context.Context,
	projectID kernel.ProjectID,
	branch string,
	imageRepository string,
	imageTag string,
	gitRefType string,
	variables map[string]string,
) (*model.Build, error) {
	auto, err := h.checker.ProjectAutomationContext(ctx, projectID)
	if err != nil {
		return nil, err
	}
	projectName, err := h.checker.ProjectName(ctx, projectID)
	if err != nil {
		return nil, err
	}
	namespace, err := h.checker.ProjectNamespace(ctx, projectID)
	if err != nil {
		return nil, err
	}
	namespace = strings.TrimSpace(namespace)
	projectName = strings.TrimSpace(projectName)
	refType := strings.TrimSpace(gitRefType)
	switch refType {
	case "tag", "pr":
	default:
		refType = "branch"
	}
	req := port.BuildRequest{
		Namespace:       namespace,
		ProjectName:     projectName,
		Folder:          namespace,
		Job:             projectName,
		GitRefType:      refType,
		GitRefValue:     branch,
		ImageRepository: strings.TrimSpace(imageRepository),
		ImageTag:        strings.TrimSpace(imageTag),
		Variables:       variables,
	}
	return h.trigger(ctx, auto.WorkspaceID, auto.MemberID, projectID, req, false, nil)
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
		if updateErr := h.persistTerminalBuild(h.workerCtx, build, projectName, memberID, wsID, projectID); updateErr != nil {
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

	if err := h.persistTerminalBuild(h.workerCtx, build, projectName, memberID, wsID, projectID); err != nil {
		h.hub.Complete(build.ID, build.ConsoleLog)
		h.reportAsyncError(fmt.Errorf("persist completed build: %w", err))
		return
	}
	h.hub.Complete(build.ID, build.ConsoleLog)
}

func (h *TriggerBuildHandler) persistTerminalBuild(
	ctx context.Context,
	build *model.Build,
	projectName string,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) error {
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.builds.Update(txCtx, build); err != nil {
			return err
		}
		return h.bus.Publish(txCtx, event.NewBuildCompleted(
			build.ID, projectID, memberID, wsID, projectName, string(build.Status), build.BuildNumber,
		))
	})
}

func (h *TriggerBuildHandler) reportAsyncError(err error) {
	if h.onAsyncError != nil {
		h.onAsyncError(err)
	}
}
