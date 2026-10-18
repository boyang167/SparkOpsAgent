package model

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// EnvironmentSyncProjectOption describes per-project sync toggles for one environment sync run.
type EnvironmentSyncProjectOption struct {
	ProjectID         kernel.ProjectID `json:"project_id"`
	SyncImage         bool             `json:"sync_image"`
	SyncYAML          bool             `json:"sync_yaml"`
	SyncResourceKinds []string          `json:"sync_resource_kinds,omitempty"`
	ResourceOverrides map[string]string `json:"resource_overrides,omitempty"`
	FirstSync         bool              `json:"first_sync"`
}

// EnvironmentSyncResource is one live Kubernetes resource captured from a cluster.
type EnvironmentSyncResource struct {
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	YAML              string `json:"yaml,omitempty"`
	ManagedPullSecret bool   `json:"managed_pull_secret,omitempty"`
}

// EnvironmentSyncDiff summarizes one planned resource change for preview.
type EnvironmentSyncDiff struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Action string `json:"action"` // create | update | unchanged | excluded
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Note   string `json:"note,omitempty"`
}

// EnvironmentSyncPlan is the deterministic merge output for one project.
type EnvironmentSyncPlan struct {
	Manifests       []SyncManifest        `json:"manifests"`
	Diffs           []EnvironmentSyncDiff `json:"diffs"`
	ImageRepository string                `json:"image_repository,omitempty"`
	ImageTag        string                `json:"image_tag,omitempty"`
}

// EnvironmentSyncProjectSnapshot captures immutable source/target inputs and the merged plan.
type EnvironmentSyncProjectSnapshot struct {
	ProjectID       kernel.ProjectID             `json:"project_id"`
	Option          EnvironmentSyncProjectOption `json:"option"`
	SourceResources []EnvironmentSyncResource    `json:"source_resources"`
	TargetResources []EnvironmentSyncResource    `json:"target_resources,omitempty"`
	Plan            EnvironmentSyncPlan          `json:"plan"`
}

// EnvironmentSyncSnapshot is an immutable preview captured before a workflow run starts.
type EnvironmentSyncSnapshot struct {
	ID                  kernel.EnvironmentSyncSnapshotID
	WorkspaceID         kernel.WorkspaceID
	ProductID           kernel.ProductID
	WorkflowID          kernel.ProductWorkflowID
	JobName             string
	SourceEnvironmentID kernel.EnvironmentID
	TargetEnvironmentID kernel.EnvironmentID
	Projects            []EnvironmentSyncProjectSnapshot
	Checksum            string
	CreatedBy           kernel.MemberID
	BoundRunID          *kernel.ProductWorkflowRunID
	CreatedAt           time.Time
	BoundAt             *time.Time
}
