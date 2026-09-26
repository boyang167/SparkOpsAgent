package model

import (
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanTemplateSnapshot struct {
	Description         string         `json:"description,omitempty"`
	ApprovalConfig      ApprovalConfig `json:"approval_config"`
	ScheduleExecuteTime *time.Time     `json:"schedule_execute_time,omitempty"`
	Jobs                []ReleasePlanTemplateJobSnapshot `json:"jobs"`
}

type ReleasePlanTemplateJobSnapshot struct {
	Name        string             `json:"name"`
	Type        ReleasePlanJobType `json:"type"`
	SortOrder   int                `json:"sort_order"`
	TextContent string             `json:"text_content,omitempty"`
	TextFormat  string             `json:"text_format,omitempty"`
	WorkflowRef json.RawMessage    `json:"workflow_ref,omitempty"`
}

type ReleasePlanTemplate struct {
	ID          kernel.ReleasePlanTemplateID
	WorkspaceID kernel.WorkspaceID
	Name        string
	Description string
	Snapshot    ReleasePlanTemplateSnapshot
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewReleasePlanTemplate(wsID kernel.WorkspaceID, name, description string, snapshot ReleasePlanTemplateSnapshot, createdBy kernel.MemberID) *ReleasePlanTemplate {
	now := time.Now().UTC()
	return &ReleasePlanTemplate{
		ID: kernel.NewReleasePlanTemplateID(), WorkspaceID: wsID,
		Name: name, Description: description, Snapshot: snapshot,
		CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
}

func SnapshotFromPlan(plan *ReleasePlan) ReleasePlanTemplateSnapshot {
	cfg := plan.ApprovalConfig
	cfg.Instance = nil
	snap := ReleasePlanTemplateSnapshot{
		Description: plan.Description, ApprovalConfig: cfg,
		ScheduleExecuteTime: plan.ScheduleExecuteTime,
		Jobs:                make([]ReleasePlanTemplateJobSnapshot, 0, len(plan.Jobs)),
	}
	for _, job := range plan.sortedJobs() {
		snap.Jobs = append(snap.Jobs, ReleasePlanTemplateJobSnapshot{
			Name: job.Name, Type: job.Type, SortOrder: job.SortOrder,
			TextContent: job.TextContent, TextFormat: job.TextFormat, WorkflowRef: job.WorkflowRef,
		})
	}
	return snap
}
