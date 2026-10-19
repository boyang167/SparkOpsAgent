package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const ProjectCreatedName = "workspace_app.ProjectCreated"

type ProjectCreated struct {
	events.BaseEvent
	ProjectID   kernel.ProjectID
	WorkspaceID kernel.WorkspaceID
	CreatedBy   kernel.MemberID
}

func NewProjectCreated(projectID kernel.ProjectID, workspaceID kernel.WorkspaceID, createdBy kernel.MemberID) ProjectCreated {
	return ProjectCreated{
		BaseEvent:   events.NewBaseEvent(ProjectCreatedName),
		ProjectID:   projectID,
		WorkspaceID: workspaceID,
		CreatedBy:   createdBy,
	}
}

func Register(registry *events.Registry) error {
	if err := events.RegisterJSON[ProjectCreated](
		registry,
		ProjectCreatedName,
		1,
		func(ev ProjectCreated) events.AggregateRef {
			return events.AggregateRef{Type: "project", ID: ev.ProjectID.String()}
		},
	); err != nil {
		return err
	}
	return registerProjectReviewSubmitted(registry)
}
