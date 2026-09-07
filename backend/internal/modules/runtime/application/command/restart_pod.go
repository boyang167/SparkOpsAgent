package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RestartPodHandler struct {
	checker   port.ProjectChecker
	lister    port.PodLister
	restarter port.PodRestarter
}

func NewRestartPodHandler(
	checker port.ProjectChecker,
	lister port.PodLister,
	restarter port.PodRestarter,
) *RestartPodHandler {
	return &RestartPodHandler{checker: checker, lister: lister, restarter: restarter}
}

func (h *RestartPodHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	podID kernel.PodID,
) error {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return err
	}
	if !inWs {
		return domain.ErrProjectAccessDenied
	}
	projectName, err := h.checker.ProjectName(ctx, projectID)
	if err != nil {
		return err
	}
	pod, err := h.lister.FindByID(ctx, projectID, projectName, podID)
	if err != nil {
		return err
	}
	return h.restarter.Restart(ctx, projectID, projectName, pod)
}
