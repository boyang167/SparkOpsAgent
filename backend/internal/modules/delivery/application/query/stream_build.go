package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type StreamBuildLogsHandler struct {
	builds  repository.BuildRepository
	checker port.ProjectChecker
	hub     *stream.Hub
}

func NewStreamBuildLogsHandler(
	builds repository.BuildRepository,
	checker port.ProjectChecker,
	hub *stream.Hub,
) *StreamBuildLogsHandler {
	return &StreamBuildLogsHandler{builds: builds, checker: checker, hub: hub}
}

func (h *StreamBuildLogsHandler) Subscribe(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	buildID kernel.BuildID,
) (<-chan string, string, bool, error) {
	build, err := h.builds.FindByID(ctx, buildID)
	if err != nil {
		return nil, "", false, err
	}
	inWs, err := h.checker.ProjectInWorkspace(ctx, build.ProjectID, wsID)
	if err != nil {
		return nil, "", false, err
	}
	if !inWs {
		return nil, "", false, domain.ErrProjectAccessDenied
	}
	ch, output, done := h.hub.Subscribe(buildID)
	if done && output == "" {
		output = build.ConsoleLog
	}
	return ch, output, done, nil
}
