package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const BuildCompletedName = "delivery.BuildCompleted"

type BuildCompleted struct {
	events.BaseEvent
	BuildID     kernel.BuildID
	ProjectID   kernel.ProjectID
	MemberID    kernel.MemberID
	WorkspaceID kernel.WorkspaceID
	ProjectName string
	Status      string
	BuildNumber int
}

func NewBuildCompleted(
	buildID kernel.BuildID,
	projectID kernel.ProjectID,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	projectName string,
	status string,
	buildNumber int,
) BuildCompleted {
	return BuildCompleted{
		BaseEvent:   events.NewBaseEvent(BuildCompletedName),
		BuildID:     buildID,
		ProjectID:   projectID,
		MemberID:    memberID,
		WorkspaceID: wsID,
		ProjectName: projectName,
		Status:      status,
		BuildNumber: buildNumber,
	}
}
