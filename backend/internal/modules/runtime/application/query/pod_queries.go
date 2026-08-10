package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PodView struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Phase     string `json:"phase"`
	Node      string `json:"node"`
	Restarts  int    `json:"restarts"`
	Container string `json:"container"`
	Image     string `json:"image"`
	StartedAt string `json:"started_at"`
}

type ListPodsHandler struct {
	checker port.ProjectChecker
	lister  port.PodLister
}

func NewListPodsHandler(checker port.ProjectChecker, lister port.PodLister) *ListPodsHandler {
	return &ListPodsHandler{checker: checker, lister: lister}
}

func (h *ListPodsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]PodView, error) {
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
	pods, err := h.lister.ListByProject(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	out := make([]PodView, 0, len(pods))
	for _, pod := range pods {
		out = append(out, toPodView(pod))
	}
	return out, nil
}

type GetPodHandler struct {
	checker port.ProjectChecker
	lister  port.PodLister
}

func NewGetPodHandler(checker port.ProjectChecker, lister port.PodLister) *GetPodHandler {
	return &GetPodHandler{checker: checker, lister: lister}
}

func (h *GetPodHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, podID kernel.PodID) (*PodView, error) {
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
	pod, err := h.lister.FindByID(ctx, projectID, projectName, podID)
	if err != nil {
		return nil, err
	}
	view := toPodView(pod)
	return &view, nil
}

func toPodView(pod *model.Pod) PodView {
	return PodView{
		ID: pod.ID.String(), ProjectID: pod.ProjectID.String(),
		Name: pod.Name, Namespace: pod.Namespace, Phase: string(pod.Phase),
		Node: pod.Node, Restarts: pod.Restarts, Container: pod.Container,
		Image: pod.Image, StartedAt: pod.StartedAt,
	}
}
