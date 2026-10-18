package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusPreparing Status = "preparing"
	StatusBuilding  Status = "building"
	StatusDeploying Status = "deploying"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusAborted   Status = "aborted"
)

type Stage string

const (
	StagePreparing Stage = "preparing"
	StageBuilding  Stage = "building"
	StageDeploying Stage = "deploying"
)

type RunType string

const (
	RunTypeNormal   RunType = "normal"
	RunTypeRollback RunType = "rollback"
	RunTypeSync     RunType = "sync"
	RunTypeUndeploy        RunType = "undeploy"
	RunTypeEnvironmentSync RunType = "environment-sync"
)

type UndeployOptions struct {
	DeleteConfig  bool `json:"delete_config"`
	DeleteStorage bool `json:"delete_storage"`
}

type DeploymentRun struct {
	ID                kernel.DeploymentRunID
	WorkspaceID       kernel.WorkspaceID
	ProjectID         kernel.ProjectID
	ClusterID         kernel.ClusterInfoID
	EnvironmentID             *kernel.EnvironmentID
	SourceEnvironmentID       *kernel.EnvironmentID
	EnvironmentSyncSnapshotID *kernel.EnvironmentSyncSnapshotID
	ProjectVersionID          *kernel.ProjectVersionID
	DeployNamespace   string
	BuildID           *kernel.BuildID
	GitRefType        string
	GitRefValue       string
	ImageTag          string
	ImageRepository   string
	CrossCluster      bool
	SyncManifests     []SyncManifest
	Status            Status
	CurrentStage      Stage
	RunType           RunType
	RollbackOfRunID   *kernel.DeploymentRunID
	UndeployOptions   *UndeployOptions
	ArchiveGeneration string
	ErrorMessage      string
	CreatedBy         kernel.MemberID
	CreatedAt         time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time
	Version           int64
	AttemptCount      int
	LeaseOwner        string
	LeaseExpiresAt    *time.Time
}

type DeploymentRunEvent struct {
	ID      uint64
	RunID   kernel.DeploymentRunID
	At      time.Time
	Status  Status
	Stage   Stage
	Message string
}

func DefaultImageTag(gitRefValue string) string {
	ref := strings.TrimSpace(gitRefValue)
	ref = strings.TrimPrefix(ref, "refs/heads/")
	ref = strings.TrimPrefix(ref, "refs/tags/")
	if ref == "" {
		return "latest"
	}

	var b strings.Builder
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == '/':
			b.WriteByte('-')
		default:
			b.WriteByte('-')
		}
	}
	tag := strings.Trim(b.String(), ".-_")
	if tag == "" {
		return "latest"
	}
	if len(tag) > 255 {
		return tag[:255]
	}
	return tag
}

func NormalizeGitRefType(gitRefType string) string {
	if strings.TrimSpace(gitRefType) == "tag" {
		return "tag"
	}
	return "branch"
}

func NewDeploymentRun(
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
	createdBy kernel.MemberID,
	gitRefType string,
	gitRefValue string,
	imageTag string,
) *DeploymentRun {
	id := kernel.NewDeploymentRunID()
	tag := strings.TrimSpace(imageTag)
	if tag == "" {
		tag = DefaultImageTag(gitRefValue)
	}
	return &DeploymentRun{
		ID:          id,
		WorkspaceID: wsID,
		ProjectID:   projectID,
		ClusterID:   clusterID,
		CreatedBy:   createdBy,
		GitRefType:  NormalizeGitRefType(gitRefType),
		GitRefValue: gitRefValue,
		ImageTag:    tag,
		Status:      StatusQueued,
		RunType:     RunTypeNormal,
		CreatedAt:   time.Now().UTC(),
	}
}

func NewRollbackDeploymentRun(
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	createdBy kernel.MemberID,
	target *DeploymentRun,
) *DeploymentRun {
	targetID := target.ID
	return &DeploymentRun{
		ID:              kernel.NewDeploymentRunID(),
		WorkspaceID:     wsID,
		ProjectID:       projectID,
		ClusterID:       target.ClusterID,
		CreatedBy:       createdBy,
		GitRefType:      target.GitRefType,
		GitRefValue:     target.GitRefValue,
		ImageTag:        target.ImageTag,
		ImageRepository: target.ImageRepository,
		CrossCluster:    target.CrossCluster,
		Status:          StatusQueued,
		RunType:         RunTypeRollback,
		RollbackOfRunID: &targetID,
		CreatedAt:       time.Now().UTC(),
	}
}

func NewSyncDeploymentRun(
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
	createdBy kernel.MemberID,
	gitRefType string,
	gitRefValue string,
	imageTag string,
) *DeploymentRun {
	tag := strings.TrimSpace(imageTag)
	if tag == "" {
		tag = DefaultImageTag(gitRefValue)
	}
	return &DeploymentRun{
		ID:           kernel.NewDeploymentRunID(),
		WorkspaceID:  wsID,
		ProjectID:    projectID,
		ClusterID:    clusterID,
		CreatedBy:    createdBy,
		GitRefType:   NormalizeGitRefType(gitRefType),
		GitRefValue:  gitRefValue,
		ImageTag:     tag,
		CrossCluster: true,
		Status:       StatusQueued,
		RunType:      RunTypeSync,
		CreatedAt:    time.Now().UTC(),
	}
}

func NewEnvironmentSyncDeploymentRun(
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
	createdBy kernel.MemberID,
) *DeploymentRun {
	run := NewDeploymentRun(wsID, projectID, clusterID, createdBy, "branch", "", "")
	run.RunType = RunTypeEnvironmentSync
	run.ImageTag = ""
	return run
}

func NewUndeployRun(
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	createdBy kernel.MemberID,
	baseline *DeploymentRun,
	opts UndeployOptions,
) *DeploymentRun {
	options := opts
	return &DeploymentRun{
		ID:               kernel.NewDeploymentRunID(),
		WorkspaceID:      wsID,
		ProjectID:        projectID,
		ClusterID:        baseline.ClusterID,
		EnvironmentID:    baseline.EnvironmentID,
		ProjectVersionID: baseline.ProjectVersionID,
		DeployNamespace:  baseline.DeployNamespace,
		CreatedBy:        createdBy,
		GitRefType:       baseline.GitRefType,
		GitRefValue:      baseline.GitRefValue,
		ImageTag:         baseline.ImageTag,
		ImageRepository:  baseline.ImageRepository,
		Status:           StatusQueued,
		RunType:          RunTypeUndeploy,
		UndeployOptions:  &options,
		CreatedAt:        time.Now().UTC(),
	}
}

func (r *DeploymentRun) IsRollback() bool {
	return r.RunType == RunTypeRollback
}

func (r *DeploymentRun) IsSyncOnly() bool {
	return r.RunType == RunTypeSync
}

func (r *DeploymentRun) IsUndeploy() bool {
	return r.RunType == RunTypeUndeploy
}

func (r *DeploymentRun) IsEnvironmentSync() bool {
	return r.RunType == RunTypeEnvironmentSync
}

func (r *DeploymentRun) SkipsBuild() bool {
	return r.IsRollback() || r.IsSyncOnly() || r.IsUndeploy() || r.IsEnvironmentSync()
}

func (r *DeploymentRun) Transition(to Status, at time.Time) error {
	if !validTransitions[r.Status][to] {
		return domain.ErrInvalidDeploymentTransition
	}

	transitionAt := at.UTC()
	r.Status = to
	r.AttemptCount = 0
	if r.StartedAt == nil {
		r.StartedAt = &transitionAt
	}
	switch to {
	case StatusPreparing:
		r.CurrentStage = StagePreparing
	case StatusBuilding:
		r.CurrentStage = StageBuilding
	case StatusDeploying:
		r.CurrentStage = StageDeploying
	}
	if isTerminalStatus(to) {
		r.FinishedAt = &transitionAt
	}
	return nil
}

func (r *DeploymentRun) Fail(msg string, at time.Time) {
	if err := r.Transition(StatusFailed, at); err != nil {
		return
	}
	r.ErrorMessage = msg
}

func isTerminalStatus(status Status) bool {
	return status == StatusSucceeded || status == StatusFailed || status == StatusAborted
}

var validTransitions = map[Status]map[Status]bool{
	StatusQueued: {
		StatusPreparing: true,
		StatusDeploying: true,
		StatusFailed:    true,
		StatusAborted:   true,
	},
	StatusPreparing: {
		StatusBuilding: true,
		StatusFailed:   true,
		StatusAborted:  true,
	},
	StatusBuilding: {
		StatusDeploying: true,
		StatusFailed:    true,
		StatusAborted:   true,
	},
	StatusDeploying: {
		StatusSucceeded: true,
		StatusFailed:    true,
		StatusAborted:   true,
	},
}
