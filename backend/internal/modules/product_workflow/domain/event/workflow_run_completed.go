package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const WorkflowRunCompletedName = "product_workflow.WorkflowRunCompleted"

type WorkflowRunCompleted struct {
	events.BaseEvent
	RunID       kernel.ProductWorkflowRunID
	WorkflowID  kernel.ProductWorkflowID
	ProductID   kernel.ProductID
	WorkspaceID kernel.WorkspaceID
	Status      string
	ErrorMessage string
}

func NewWorkflowRunCompleted(
	runID kernel.ProductWorkflowRunID,
	workflowID kernel.ProductWorkflowID,
	productID kernel.ProductID,
	wsID kernel.WorkspaceID,
	status string,
	errorMessage string,
) WorkflowRunCompleted {
	return WorkflowRunCompleted{
		BaseEvent:    events.NewBaseEvent(WorkflowRunCompletedName),
		RunID:        runID,
		WorkflowID:   workflowID,
		ProductID:    productID,
		WorkspaceID:  wsID,
		Status:       status,
		ErrorMessage: errorMessage,
	}
}
