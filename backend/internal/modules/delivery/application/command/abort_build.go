package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AbortBuildHandler struct {
	builds   repository.BuildRepository
	checker  port.ProjectChecker
	registry *runner.Registry
	hub      *stream.Hub
}

func NewAbortBuildHandler(
	builds repository.BuildRepository,
	checker port.ProjectChecker,
	registry *runner.Registry,
	hub *stream.Hub,
) *AbortBuildHandler {
	return &AbortBuildHandler{builds: builds, checker: checker, registry: registry, hub: hub}
}

func (h *AbortBuildHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, buildID kernel.BuildID) error {
	build, err := h.builds.FindByID(ctx, buildID)
	if err != nil {
		return err
	}
	inWs, err := h.checker.ProjectInWorkspace(ctx, build.ProjectID, wsID)
	if err != nil {
		return err
	}
	if !inWs {
		return domain.ErrProjectAccessDenied
	}
	if !build.IsRunning() {
		return domain.ErrBuildNotRunning
	}
	// No registered executor means the tracking goroutine is gone (e.g. process restart);
	// the persisted running status is stale and must still be closed out.
	h.registry.Cancel(build.ID.String())
	build.Abort(build.ConsoleLog)
	if err := h.builds.Update(ctx, build); err != nil {
		return err
	}
	h.hub.Complete(build.ID, build.ConsoleLog)
	return nil
}
