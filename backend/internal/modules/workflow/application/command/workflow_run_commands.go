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
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type StartWorkflowRunHandler struct {
	workflows repository.WorkflowRepository
	runs      repository.WorkflowRunRepository
	checker   port.ProjectChecker
	bus       eventbus.Bus
	tx        postgres.Transactor
}

func NewStartWorkflowRunHandler(
	workflows repository.WorkflowRepository,
	runs repository.WorkflowRunRepository,
	checker port.ProjectChecker,
	bus eventbus.Bus,
	tx postgres.Transactor,
) *StartWorkflowRunHandler {
	return &StartWorkflowRunHandler{workflows: workflows, runs: runs, checker: checker, bus: bus, tx: tx}
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
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.runs.Save(txCtx, run); err != nil {
			return err
		}
		output, execErr := engine.Execute(workflow.Definition)
		if execErr != nil {
			run.Fail(execErr.Error())
		} else {
			run.Complete(output)
		}
		if err := h.runs.Update(txCtx, run); err != nil {
			return err
		}
		return h.bus.Publish(txCtx, event.NewWorkflowRunCompleted(
			run.ID,
			workflow.ID,
			workflow.ProjectID,
			memberID,
			wsID,
			workflow.Name.String(),
			string(run.Status),
		))
	})
	if err != nil {
		return nil, err
	}
	return run, nil
}
