package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const DeployCompletedName = "delivery.DeployCompleted"

type DeployCompleted struct {
	events.BaseEvent
	RunID       kernel.DeploymentRunID
	ProjectID   kernel.ProjectID
	WorkspaceID kernel.WorkspaceID
	ProjectName string
	Status      string
	RunType     string
	ImageTag    string
}

func NewDeployCompleted(
	runID kernel.DeploymentRunID,
	projectID kernel.ProjectID,
	wsID kernel.WorkspaceID,
	projectName string,
	status string,
	runType string,
	imageTag string,
) DeployCompleted {
	return DeployCompleted{
		BaseEvent:   events.NewBaseEvent(DeployCompletedName),
		RunID:       runID,
		ProjectID:   projectID,
		WorkspaceID: wsID,
		ProjectName: projectName,
		Status:      status,
		RunType:     runType,
		ImageTag:    imageTag,
	}
}
