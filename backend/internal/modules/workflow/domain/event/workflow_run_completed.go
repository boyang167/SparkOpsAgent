package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const WorkflowRunCompletedName = "workflow.WorkflowRunCompleted"

type WorkflowRunCompleted struct {
	events.BaseEvent
	RunID       kernel.WorkflowRunID
	WorkflowID  kernel.WorkflowID
	ProjectID   kernel.ProjectID
	MemberID    kernel.MemberID
	WorkspaceID kernel.WorkspaceID
	WorkflowName string
	Status      string
}

func NewWorkflowRunCompleted(
	runID kernel.WorkflowRunID,
	workflowID kernel.WorkflowID,
	projectID kernel.ProjectID,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	workflowName string,
	status string,
) WorkflowRunCompleted {
	return WorkflowRunCompleted{
		BaseEvent:    events.NewBaseEvent(WorkflowRunCompletedName),
		RunID:        runID,
		WorkflowID:   workflowID,
		ProjectID:    projectID,
		MemberID:     memberID,
		WorkspaceID:  wsID,
		WorkflowName: workflowName,
		Status:       status,
	}
}
