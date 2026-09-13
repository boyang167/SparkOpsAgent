package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GetPodYAMLHandler struct {
	checker   port.ProjectChecker
	inspector *runtimek8s.ProjectRuntimeInspector
}

func NewGetPodYAMLHandler(
	checker port.ProjectChecker,
	inspector *runtimek8s.ProjectRuntimeInspector,
) *GetPodYAMLHandler {
	return &GetPodYAMLHandler{checker: checker, inspector: inspector}
}

func (h *GetPodYAMLHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	podID kernel.PodID,
) (string, error) {
	inWorkspace, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return "", err
	}
	if !inWorkspace {
		return "", domain.ErrProjectAccessDenied
	}
	if h.inspector == nil {
		return "", domain.ErrPodNotFound
	}
	return h.inspector.PodYAML(ctx, wsID, projectID, podID)
}
