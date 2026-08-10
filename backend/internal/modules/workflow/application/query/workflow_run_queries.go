package query

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowRunView struct {
	ID          string  `json:"id"`
	WorkflowID  string  `json:"workflow_id"`
	Status      string  `json:"status"`
	Input       string  `json:"input"`
	Output      string  `json:"output"`
	Error       string  `json:"error,omitempty"`
	StartedAt   string  `json:"started_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

type ListWorkflowRunsHandler struct {
	workflows repository.WorkflowRepository
	runs      repository.WorkflowRunRepository
	checker   port.ProjectChecker
}

func NewListWorkflowRunsHandler(
	workflows repository.WorkflowRepository,
	runs repository.WorkflowRunRepository,
	checker port.ProjectChecker,
) *ListWorkflowRunsHandler {
	return &ListWorkflowRunsHandler{workflows: workflows, runs: runs, checker: checker}
}

func (h *ListWorkflowRunsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, workflowID kernel.WorkflowID) ([]WorkflowRunView, error) {
	workflow, err := h.workflows.FindByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.checker.ProjectInWorkspace(ctx, workflow.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	if workflow.Status == model.WorkflowStatusArchived {
		return nil, domain.ErrWorkflowNotFound
	}
	runs, err := h.runs.ListByWorkflow(ctx, workflowID, 20)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowRunView, 0, len(runs))
	for _, run := range runs {
		out = append(out, toRunView(run))
	}
	return out, nil
}

type GetWorkflowRunHandler struct {
	workflows repository.WorkflowRepository
	runs      repository.WorkflowRunRepository
	checker   port.ProjectChecker
}

func NewGetWorkflowRunHandler(
	workflows repository.WorkflowRepository,
	runs repository.WorkflowRunRepository,
	checker port.ProjectChecker,
) *GetWorkflowRunHandler {
	return &GetWorkflowRunHandler{workflows: workflows, runs: runs, checker: checker}
}

func (h *GetWorkflowRunHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, runID kernel.WorkflowRunID) (*WorkflowRunView, error) {
	run, err := h.runs.FindByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	workflow, err := h.workflows.FindByID(ctx, run.WorkflowID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.checker.ProjectInWorkspace(ctx, workflow.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	view := toRunView(run)
	return &view, nil
}

func toRunView(run *model.WorkflowRun) WorkflowRunView {
	view := WorkflowRunView{
		ID: run.ID.String(), WorkflowID: run.WorkflowID.String(),
		Status: string(run.Status), Input: run.Input,
		Output: run.Output, Error: run.Error,
		StartedAt: run.StartedAt.Format(time.RFC3339),
	}
	if run.CompletedAt != nil {
		s := run.CompletedAt.Format(time.RFC3339)
		view.CompletedAt = &s
	}
	return view
}
