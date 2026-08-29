package command

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RollbackBuildHandler struct {
	trigger *TriggerBuildHandler
	builds  repository.BuildRepository
	checker port.ProjectChecker
}

func NewRollbackBuildHandler(
	builds repository.BuildRepository,
	checker port.ProjectChecker,
	jenkins port.JenkinsExecutor,
	bus eventbus.Bus,
	hub *stream.Hub,
	registry *runner.Registry,
	workerCtx context.Context,
) *RollbackBuildHandler {
	return &RollbackBuildHandler{
		trigger: NewTriggerBuildHandler(builds, checker, jenkins, bus, hub, registry, workerCtx),
		builds:  builds,
		checker: checker,
	}
}

func (h *RollbackBuildHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	sourceBuildID kernel.BuildID,
) (*model.Build, error) {
	source, err := h.builds.FindByID(ctx, sourceBuildID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.checker.ProjectInWorkspace(ctx, source.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	if source.Status != model.BuildStatusSuccess {
		return nil, domain.ErrBuildNotRollbackable
	}
	running, err := h.builds.HasRunningBuild(ctx, source.ProjectID)
	if err != nil {
		return nil, err
	}
	if running {
		return nil, domain.ErrBuildAlreadyRunning
	}

	projectName, err := h.checker.ProjectName(ctx, source.ProjectID)
	if err != nil {
		return nil, err
	}

	buildNumber, err := h.builds.NextBuildNumber(ctx, source.ProjectID)
	if err != nil {
		return nil, err
	}

	build := model.NewBuild(source.ProjectID, memberID, wsID, source.Branch)
	build.BuildNumber = buildNumber
	build.JenkinsJob = fmt.Sprintf("%s-build", projectName)
	build.RollbackOfBuildID = source.ID.String()
	if err := h.builds.Save(ctx, build); err != nil {
		return nil, err
	}

	go h.trigger.executeBuild(build, projectName, port.BuildRequest{
		Job:         projectName + "-build",
		GitRefValue: source.Branch,
		RollbackOf:  source.BuildNumber,
	}, memberID, wsID, source.ProjectID)
	return build, nil
}
