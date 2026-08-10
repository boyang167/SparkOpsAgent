package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowView struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Definition  string `json:"definition"`
	CreatedAt   string `json:"created_at"`
}

type ListWorkflowsHandler struct {
	repo           repository.WorkflowRepository
	projectChecker port.ProjectChecker
}

func NewListWorkflowsHandler(repo repository.WorkflowRepository, projectChecker port.ProjectChecker) *ListWorkflowsHandler {
	return &ListWorkflowsHandler{repo: repo, projectChecker: projectChecker}
}

func (h *ListWorkflowsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]WorkflowView, error) {
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	workflows, err := h.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowView, 0, len(workflows))
	for _, wf := range workflows {
		out = append(out, toView(wf))
	}
	return out, nil
}

type GetWorkflowHandler struct {
	repo           repository.WorkflowRepository
	projectChecker port.ProjectChecker
}

func NewGetWorkflowHandler(repo repository.WorkflowRepository, projectChecker port.ProjectChecker) *GetWorkflowHandler {
	return &GetWorkflowHandler{repo: repo, projectChecker: projectChecker}
}

func (h *GetWorkflowHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, workflowID kernel.WorkflowID) (*WorkflowView, error) {
	workflow, err := h.repo.FindByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, workflow.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	if workflow.Status == model.WorkflowStatusArchived {
		return nil, domain.ErrWorkflowNotFound
	}
	view := toView(workflow)
	return &view, nil
}

func toView(wf *model.Workflow) WorkflowView {
	return WorkflowView{
		ID: wf.ID.String(), ProjectID: wf.ProjectID.String(),
		Name: wf.Name.String(), Description: wf.Description,
		Status: string(wf.Status), Definition: wf.Definition,
		CreatedAt: wf.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
