package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintWorkItemExecutor struct {
	products   repository.ProductRepository
	sprints    repository.SprintRepository
	trigger    appctx.WorkflowRunTrigger
	pm         *port.PmConnectorRouter
	board      *SprintBoardNotifier
	operations messaging.OperationWriter
}

func (e *SprintWorkItemExecutor) UseOperationWriter(writer messaging.OperationWriter) {
	e.operations = writer
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
	if success && model.WorkItemJiraSyncMode(item) == "bidirectional" {
		issueKey := model.WorkItemPmIssueKey(item)
		provider := model.WorkItemPmProvider(item)
		pmWorkspaceID := model.WorkItemPmWorkspaceID(item)
		if issueKey != "" && e.products != nil {
			sprint, err := e.sprints.FindSprintByID(ctx, item.SprintID)
			if err != nil {
				return err
			}
			product, err := e.products.FindProductByID(ctx, sprint.ProductID)
			if err != nil {
				return err
			}
			if e.operations != nil {
				if err := scheduleJiraTransition(ctx, e.operations, productoutbound.JiraTransitionPayload{
					WorkspaceID: product.WorkspaceID.String(), PMWorkspaceID: pmWorkspaceID,
					Provider: provider, IssueKey: issueKey, Status: "Done",
				}); err != nil {
					return err
				}
			} else {
				if e.pm == nil {
					return fmt.Errorf("pm connector router unavailable")
				}
				connector := e.pm.Get(provider)
				if connector == nil {
					return fmt.Errorf("pm connector %q unavailable", provider)
				}
				if err := connector.TransitionIssue(ctx, product.WorkspaceID, pmWorkspaceID, issueKey, "Done"); err != nil {
					return err
				}
			}
		}
	}
	if e.board != nil {
		e.board.BoardChanged(item.SprintID, "execution_completed")
	}
	return nil
}
