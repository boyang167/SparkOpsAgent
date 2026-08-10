package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/infrastructure/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type StartWorkflowRunHandler struct {
	workflows repository.WorkflowRepository
	runs      repository.WorkflowRunRepository
	checker   port.ProjectChecker
	bus       eventbus.Bus
}

func NewStartWorkflowRunHandler(
	workflows repository.WorkflowRepository,
	runs repository.WorkflowRunRepository,
	checker port.ProjectChecker,
	bus eventbus.Bus,
) *StartWorkflowRunHandler {
	return &StartWorkflowRunHandler{workflows: workflows, runs: runs, checker: checker, bus: bus}
}

func (h *StartWorkflowRunHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	workflowID kernel.WorkflowID,
	input string,
) (*model.WorkflowRun, error) {
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
	if err := workflow.EnsureRunnable(); err != nil {
		return nil, err
	}

	run := model.NewWorkflowRun(workflowID, memberID, wsID, input)
	if err := h.runs.Save(ctx, run); err != nil {
		return nil, err
	}

	output, execErr := engine.Execute(workflow.Definition)
	if execErr != nil {
		run.Fail(execErr.Error())
	} else {
		run.Complete(output)
	}
	if err := h.runs.Update(ctx, run); err != nil {
		return nil, err
	}

	_ = h.bus.Publish(ctx, event.NewWorkflowRunCompleted(
		run.ID,
		workflow.ID,
		workflow.ProjectID,
		memberID,
		wsID,
		workflow.Name.String(),
		string(run.Status),
	))

	return run, nil
}
