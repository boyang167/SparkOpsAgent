package model

import (
	"encoding/json"
	"maps"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanVersionSnapshot struct {
	Name                string         `json:"name"`
	Description         string         `json:"description,omitempty"`
	ManagerID           string         `json:"manager_id,omitempty"`
	SprintID            *string        `json:"sprint_id,omitempty"`
	Status              ReleasePlanStatus `json:"status,omitempty"`
	ReleaseWindowStart  time.Time      `json:"release_window_start"`
	ReleaseWindowEnd    time.Time      `json:"release_window_end"`
	ScheduleExecuteTime *time.Time     `json:"schedule_execute_time,omitempty"`
	ApprovalConfig      ApprovalConfig `json:"approval_config"`
	Parameters          map[string]string `json:"parameters,omitempty"`
	Jobs                []ReleasePlanVersionJobSnapshot `json:"jobs"`
}

type ReleasePlanVersionJobSnapshot struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Type        ReleasePlanJobType `json:"type"`
	SortOrder   int                `json:"sort_order"`
	OwnerID     string             `json:"owner_id,omitempty"`
	TextContent string             `json:"text_content,omitempty"`
	TextFormat  string             `json:"text_format,omitempty"`
	WorkflowRef json.RawMessage    `json:"workflow_ref,omitempty"`
}

type ReleasePlanVersion struct {
	ReleasePlanID   kernel.ReleasePlanID
	Version         int
	PreviousVersion int
	ActorID         kernel.MemberID
	Verb            string
	SectionKey      string
	SectionName     string
	Snapshot        ReleasePlanVersionSnapshot
	CreatedAt       time.Time
}

func VersionSnapshotFromPlan(plan *ReleasePlan) ReleasePlanVersionSnapshot {
	cfg := plan.ApprovalConfig
	cfg.Instance = nil
	snap := ReleasePlanVersionSnapshot{
		Name: plan.Name, Description: plan.Description,
		ManagerID: plan.ManagerID.String(), Status: plan.Status,
		ReleaseWindowStart: plan.ReleaseWindowStart, ReleaseWindowEnd: plan.ReleaseWindowEnd,
		ScheduleExecuteTime: plan.ScheduleExecuteTime, ApprovalConfig: cfg,
		Parameters: maps.Clone(plan.Parameters),
		Jobs: make([]ReleasePlanVersionJobSnapshot, 0, len(plan.Jobs)),
	}
	if plan.SprintID != nil {
		v := plan.SprintID.String()
		snap.SprintID = &v
	}
	for _, job := range plan.sortedJobs() {
		snap.Jobs = append(snap.Jobs, ReleasePlanVersionJobSnapshot{
			ID: job.ID.String(), Name: job.Name, Type: job.Type, SortOrder: job.SortOrder,
			OwnerID: job.OwnerID.String(), TextContent: job.TextContent, TextFormat: job.TextFormat,
			WorkflowRef: job.WorkflowRef,
		})
	}
	return snap
}

func NewReleasePlanVersion(
	planID kernel.ReleasePlanID,
	version, previousVersion int,
	actor kernel.MemberID,
	verb, sectionKey, sectionName string,
	snapshot ReleasePlanVersionSnapshot,
) *ReleasePlanVersion {
	return &ReleasePlanVersion{
		ReleasePlanID: planID, Version: version, PreviousVersion: previousVersion,
		ActorID: actor, Verb: verb, SectionKey: sectionKey, SectionName: sectionName,
		Snapshot: snapshot, CreatedAt: time.Now().UTC(),
	}
}
