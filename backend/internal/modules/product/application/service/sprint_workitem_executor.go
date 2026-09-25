package service

import (
	"context"
	"errors"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintWorkItemExecutor struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
	trigger  appctx.WorkflowRunTrigger
	pm       *port.PmConnectorRouter
	board    *SprintBoardNotifier
}

func NewSprintWorkItemExecutor(
	products repository.ProductRepository,
	sprints repository.SprintRepository,
	trigger appctx.WorkflowRunTrigger,
	pm *port.PmConnectorRouter,
	board *SprintBoardNotifier,
) *SprintWorkItemExecutor {
	return &SprintWorkItemExecutor{products: products, sprints: sprints, trigger: trigger, pm: pm, board: board}
}

type ExecuteWorkItemResult struct {
	ExecutionID   kernel.WorkItemExecutionID
	WorkflowRunID kernel.ProductWorkflowRunID
}

func (e *SprintWorkItemExecutor) Execute(
	ctx context.Context,
	sprint *model.Sprint,
	item *model.WorkItem,
	stageID, workflowID string,
	actor kernel.MemberID,
	input map[string]any,
) (*ExecuteWorkItemResult, error) {
	if sprint.Status != model.SprintStatusActive {
		return nil, domain.ErrSprintNotActive
	}
	if stageID == "" {
		stageID = item.StageID
	}
	binding, err := sprint.ResolveStageWorkflow(stageID, workflowID)
	if err != nil {
		return nil, err
	}
	wfID, err := kernel.ParseProductWorkflowID(binding.WorkflowID)
	if err != nil {
		return nil, domain.ErrWorkflowNotConfigured
	}
	if e.trigger == nil {
		return nil, domain.ErrWorkflowNotConfigured
	}
	runInput := input
	if runInput == nil {
		runInput = map[string]any{}
	}
	if len(binding.DefaultParameters) > 0 {
		vars, _ := runInput["variables"].(map[string]any)
		if vars == nil {
			vars = map[string]any{}
		}
		for k, v := range binding.DefaultParameters {
			if _, ok := vars[k]; !ok {
				vars[k] = v
			}
		}
		runInput["variables"] = vars
	}
	runID, err := e.trigger.StartForProductWithInput(ctx, sprint.ProductID, wfID, actor, runInput)
	if err != nil {
		return nil, err
	}
	exec := model.NewWorkItemExecution(item.ID, stageID, wfID, runID, actor)
	if err := e.sprints.SaveExecution(ctx, exec); err != nil {
		return nil, err
	}
	if item.Delivery == nil {
		item.Delivery = map[string]any{}
	}
	item.Delivery["last_workflow_run_id"] = runID.String()
	item.Delivery["last_workflow_id"] = wfID.String()
	item.Delivery["last_workflow_status"] = "running"
	if branch, _ := runInput["branch"].(string); branch != "" {
		item.Delivery["branch"] = branch
	}
	item.Status = model.WorkItemStatusInProgress
	if err := e.sprints.UpdateWorkItem(ctx, item); err != nil {
		return nil, err
	}
	return &ExecuteWorkItemResult{ExecutionID: exec.ID, WorkflowRunID: runID}, nil
}

func (e *SprintWorkItemExecutor) OnWorkflowRunCompleted(
	ctx context.Context,
	runID kernel.ProductWorkflowRunID,
	success bool,
	errMsg string,
) error {
	exec, err := e.sprints.FindExecutionByWorkflowRunID(ctx, runID)
	if err != nil {
		if errors.Is(err, domain.ErrWorkItemExecutionNotFound) {
			return nil
		}
		return err
	}
	exec.Complete(success, errMsg)
	if err := e.sprints.UpdateExecution(ctx, exec); err != nil {
		return err
	}
	item, err := e.sprints.FindWorkItemByID(ctx, exec.WorkItemID)
	if err != nil {
		return err
	}
	if item.Delivery == nil {
		item.Delivery = map[string]any{}
	}
	if success {
		item.Delivery["last_workflow_status"] = "success"
	} else {
		item.Delivery["last_workflow_status"] = "failed"
		item.Delivery["last_workflow_error"] = errMsg
	}
	if err := e.sprints.UpdateWorkItem(ctx, item); err != nil {
		return err
	}
	if success && e.pm != nil && model.WorkItemJiraSyncMode(item) == "bidirectional" {
		issueKey := model.WorkItemPmIssueKey(item)
		provider := model.WorkItemPmProvider(item)
		pmWorkspaceID := model.WorkItemPmWorkspaceID(item)
		if issueKey != "" && e.products != nil {
			if connector := e.pm.Get(provider); connector != nil {
				sprint, sErr := e.sprints.FindSprintByID(ctx, item.SprintID)
				if sErr == nil {
					product, pErr := e.products.FindProductByID(ctx, sprint.ProductID)
					if pErr == nil {
						_ = connector.TransitionIssue(ctx, product.WorkspaceID, pmWorkspaceID, issueKey, "Done")
					}
				}
			}
		}
	}
	if e.board != nil {
		e.board.BoardChanged(item.SprintID, "execution_completed")
	}
	return nil
}
