package model

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkItemExecutionStatus string

const (
	WorkItemExecutionRunning   WorkItemExecutionStatus = "running"
	WorkItemExecutionSuccess   WorkItemExecutionStatus = "success"
	WorkItemExecutionFailed    WorkItemExecutionStatus = "failed"
	WorkItemExecutionCancelled WorkItemExecutionStatus = "cancelled"
)

type WorkItemExecution struct {
	ID            kernel.WorkItemExecutionID
	WorkItemID    kernel.WorkItemID
	StageID       string
	WorkflowID    kernel.ProductWorkflowID
	WorkflowRunID kernel.ProductWorkflowRunID
	TriggeredBy   kernel.MemberID
	Status        WorkItemExecutionStatus
	Artifacts     map[string]any
	ErrorMessage  string
	StartedAt     time.Time
	FinishedAt    *time.Time
}

func NewWorkItemExecution(
	workItemID kernel.WorkItemID,
	stageID string,
	workflowID kernel.ProductWorkflowID,
	runID kernel.ProductWorkflowRunID,
	triggeredBy kernel.MemberID,
) *WorkItemExecution {
	return &WorkItemExecution{
		ID:            kernel.NewWorkItemExecutionID(),
		WorkItemID:    workItemID,
		StageID:       stageID,
		WorkflowID:    workflowID,
		WorkflowRunID: runID,
		TriggeredBy:   triggeredBy,
		Status:        WorkItemExecutionRunning,
		Artifacts:     map[string]any{},
		StartedAt:     time.Now().UTC(),
	}
}

func (e *WorkItemExecution) Complete(success bool, errMsg string) {
	now := time.Now().UTC()
	e.FinishedAt = &now
	if success {
		e.Status = WorkItemExecutionSuccess
	} else {
		e.Status = WorkItemExecutionFailed
		e.ErrorMessage = errMsg
	}
}
