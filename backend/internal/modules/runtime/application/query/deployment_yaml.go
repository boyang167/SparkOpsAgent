package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GetDeploymentYAMLHandler struct {
	checker   port.ProjectChecker
	inspector *runtimek8s.ProjectRuntimeInspector
}

func NewGetDeploymentYAMLHandler(
	checker port.ProjectChecker,
	inspector *runtimek8s.ProjectRuntimeInspector,
) *GetDeploymentYAMLHandler {
	return &GetDeploymentYAMLHandler{checker: checker, inspector: inspector}
}

func (h *GetDeploymentYAMLHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (string, error) {
	inWorkspace, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return "", err
	}
	if !inWorkspace {
		return "", domain.ErrProjectAccessDenied
	}
	if h.inspector == nil {
		return "", domain.ErrResourceNotFound
	}
	return h.inspector.DeploymentYAML(ctx, wsID, projectID)
}
