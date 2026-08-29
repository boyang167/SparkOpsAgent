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

type DeploymentRun struct {
	ID             kernel.DeploymentRunID
	WorkspaceID    kernel.WorkspaceID
	ProjectID      kernel.ProjectID
	ClusterID      kernel.ClusterInfoID
	BuildID        *kernel.BuildID
	GitRefType     string
	GitRefValue    string
	ImageTag       string
	Status         Status
	CurrentStage   Stage
	ErrorMessage   string
	CreatedBy      kernel.MemberID
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	Version        int64
	AttemptCount   int
	LeaseOwner     string
	LeaseExpiresAt *time.Time
}

type DeploymentRunEvent struct {
	ID      uint64
	RunID   kernel.DeploymentRunID
	At      time.Time
	Status  Status
	Stage   Stage
	Message string
}

func DefaultImageTag(id kernel.DeploymentRunID) string {
	tag := strings.ReplaceAll(id.String(), "-", "")
	if len(tag) > 12 {
		return tag[:12]
	}
	return tag
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
		tag = DefaultImageTag(id)
	}
	return &DeploymentRun{
		ID:          id,
		WorkspaceID: wsID,
		ProjectID:   projectID,
		ClusterID:   clusterID,
		CreatedBy:   createdBy,
		GitRefType:  gitRefType,
		GitRefValue: gitRefValue,
		ImageTag:    tag,
		Status:      StatusQueued,
		CreatedAt:   time.Now().UTC(),
	}
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
