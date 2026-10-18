package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const DeployCompletedName = "delivery.DeployCompleted"

type DeployCompleted struct {
	events.BaseEvent
	RunID                     kernel.DeploymentRunID
	ProjectID                 kernel.ProjectID
	WorkspaceID               kernel.WorkspaceID
	EnvironmentID             *kernel.EnvironmentID
	SourceEnvironmentID       *kernel.EnvironmentID
	EnvironmentSyncSnapshotID *kernel.EnvironmentSyncSnapshotID
	ProjectName               string
	Status                    string
	RunType                   string
	ImageTag                  string
	CreatedBy                 kernel.MemberID
}

func NewDeployCompleted(
	runID kernel.DeploymentRunID,
	projectID kernel.ProjectID,
	wsID kernel.WorkspaceID,
	environmentID *kernel.EnvironmentID,
	projectName string,
	status string,
	runType string,
	imageTag string,
) DeployCompleted {
	return DeployCompleted{
		BaseEvent:     events.NewBaseEvent(DeployCompletedName),
		RunID:         runID,
		ProjectID:     projectID,
		WorkspaceID:   wsID,
		EnvironmentID: environmentID,
		ProjectName:   projectName,
		Status:        status,
		RunType:       runType,
		ImageTag:      imageTag,
	}
}
