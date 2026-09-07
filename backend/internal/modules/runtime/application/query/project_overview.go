package query

import (
	"context"

	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectRuntimeOverviewView struct {
	Deployment    *runtimek8s.DeploymentOverview    `json:"deployment,omitempty"`
	Service       *runtimek8s.ServiceOverview       `json:"service,omitempty"`
	LogCollection *runtimek8s.LogCollectionOverview `json:"log_collection,omitempty"`
	HealthProbe   *runtimek8s.HealthProbeOverview   `json:"health_probe,omitempty"`
	Ingress       *runtimek8s.IngressOverview       `json:"ingress,omitempty"`
	Pods          []PodView                         `json:"pods"`
}

type GetProjectRuntimeOverviewHandler struct {
	checker   port.ProjectChecker
	lister    port.PodLister
	inspector *runtimek8s.ProjectRuntimeInspector
	metrics   port.PodMetricsReader
}

func NewGetProjectRuntimeOverviewHandler(
	checker port.ProjectChecker,
	lister port.PodLister,
	inspector *runtimek8s.ProjectRuntimeInspector,
	metrics port.PodMetricsReader,
) *GetProjectRuntimeOverviewHandler {
	return &GetProjectRuntimeOverviewHandler{
		checker: checker, lister: lister, inspector: inspector, metrics: metrics,
	}
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
			out.LogCollection = overview.LogCollection
			out.HealthProbe = overview.HealthProbe
			out.Ingress = overview.Ingress
		}
	}
	pods, err := h.lister.ListByProject(ctx, projectID, projectName)
	if err != nil {
		return out, nil
	}
	metricsByPod := map[kernel.PodID]port.PodMetrics{}
	if h.metrics != nil {
		if fetched, metricsErr := h.metrics.MetricsForPods(ctx, projectID, pods); metricsErr == nil {
			metricsByPod = fetched
		}
	}
	out.Pods = make([]PodView, 0, len(pods))
	for _, pod := range pods {
		out.Pods = append(out.Pods, toPodViewWithMetrics(pod, metricsByPod[pod.ID]))
	}
	return out, nil
}
