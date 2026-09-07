package query

import (
	"context"

	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectRuntimeOverviewView struct {
	Deployment *runtimek8s.DeploymentOverview `json:"deployment,omitempty"`
	Service    *runtimek8s.ServiceOverview    `json:"service,omitempty"`
	Pods       []PodView                      `json:"pods"`
}

type GetProjectRuntimeOverviewHandler struct {
	checker   port.ProjectChecker
	lister    port.PodLister
	inspector *runtimek8s.ProjectRuntimeInspector
}

func NewGetProjectRuntimeOverviewHandler(
	checker port.ProjectChecker,
	lister port.PodLister,
	inspector *runtimek8s.ProjectRuntimeInspector,
) *GetProjectRuntimeOverviewHandler {
	return &GetProjectRuntimeOverviewHandler{checker: checker, lister: lister, inspector: inspector}
}

func (h *GetProjectRuntimeOverviewHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (*ProjectRuntimeOverviewView, error) {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	projectName, err := h.checker.ProjectName(ctx, projectID)
	if err != nil {
		return nil, err
	}

	out := &ProjectRuntimeOverviewView{Pods: []PodView{}}
	if h.inspector != nil {
		overview, inspectErr := h.inspector.Inspect(ctx, wsID, projectID)
		if inspectErr == nil && overview != nil {
			out.Deployment = overview.Deployment
			out.Service = overview.Service
		}
	}
	pods, err := h.lister.ListByProject(ctx, projectID, projectName)
	if err != nil {
		return out, nil
	}
	out.Pods = make([]PodView, 0, len(pods))
	for _, pod := range pods {
		out.Pods = append(out.Pods, toPodView(pod))
	}
	return out, nil
}
