package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func loadPod(
	ctx context.Context,
	checker port.ProjectChecker,
	lister port.PodLister,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	podID kernel.PodID,
) (*model.Pod, error) {
	inWs, err := checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	projectName, err := checker.ProjectName(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return lister.FindByID(ctx, projectID, projectName, podID)
}

type GetPodLogsHandler struct {
	checker port.ProjectChecker
	lister  port.PodLister
	logs    port.PodLogReader
}

func NewGetPodLogsHandler(
	checker port.ProjectChecker,
	lister port.PodLister,
	logs port.PodLogReader,
) *GetPodLogsHandler {
	return &GetPodLogsHandler{checker: checker, lister: lister, logs: logs}
}

func (h *GetPodLogsHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	podID kernel.PodID,
) (string, error) {
	pod, err := loadPod(ctx, h.checker, h.lister, wsID, projectID, podID)
	if err != nil {
		return "", err
	}
	return h.logs.ReadLogs(ctx, pod)
}

type StreamPodTerminalHandler struct {
	checker  port.ProjectChecker
	lister   port.PodLister
	terminal port.PodTerminal
}

func NewStreamPodTerminalHandler(
	checker port.ProjectChecker,
	lister port.PodLister,
	terminal port.PodTerminal,
) *StreamPodTerminalHandler {
	return &StreamPodTerminalHandler{checker: checker, lister: lister, terminal: terminal}
}

func (h *StreamPodTerminalHandler) Lines(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	podID kernel.PodID,
) ([]string, error) {
	pod, err := loadPod(ctx, h.checker, h.lister, wsID, projectID, podID)
	if err != nil {
		return nil, err
	}
	return h.terminal.StreamLines(ctx, pod), nil
}
