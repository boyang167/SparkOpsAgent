package model

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowRunStatus string

const (
	WorkflowRunStatusRunning   WorkflowRunStatus = "running"
	WorkflowRunStatusCompleted WorkflowRunStatus = "completed"
	WorkflowRunStatusFailed    WorkflowRunStatus = "failed"
)

type WorkflowRun struct {
	ID          kernel.WorkflowRunID
	WorkflowID  kernel.WorkflowID
	MemberID    kernel.MemberID
	WorkspaceID kernel.WorkspaceID
	Status      WorkflowRunStatus
	Input       string
	Output      string
	Error       string
	StartedAt   time.Time
	CompletedAt *time.Time
}

func NewWorkflowRun(
	workflowID kernel.WorkflowID,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	input string,
) *WorkflowRun {
	return &WorkflowRun{
		ID:          kernel.NewWorkflowRunID(),
		WorkflowID:  workflowID,
		MemberID:    memberID,
		WorkspaceID: wsID,
		Status:      WorkflowRunStatusRunning,
		Input:       input,
		StartedAt:   time.Now().UTC(),
	}
}

func (r *WorkflowRun) Complete(output string) {
	now := time.Now().UTC()
	r.Status = WorkflowRunStatusCompleted
	r.Output = output
	r.CompletedAt = &now
}

func (r *WorkflowRun) Fail(errMsg string) {
	now := time.Now().UTC()
	r.Status = WorkflowRunStatusFailed
	r.Error = errMsg
	r.CompletedAt = &now
}
