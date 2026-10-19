package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const ProjectReviewSubmittedName = "workspace_app.ProjectReviewSubmitted"

type ProjectReviewSubmitted struct {
	events.BaseEvent
	ProjectID   kernel.ProjectID
	WorkspaceID kernel.WorkspaceID
	ProjectName string
	DisplayName string
	SubmittedBy kernel.MemberID
}

func NewProjectReviewSubmitted(
	projectID kernel.ProjectID,
	workspaceID kernel.WorkspaceID,
	projectName, displayName string,
	submittedBy kernel.MemberID,
) ProjectReviewSubmitted {
	return ProjectReviewSubmitted{
		BaseEvent:   events.NewBaseEvent(ProjectReviewSubmittedName),
		ProjectID:   projectID,
		WorkspaceID: workspaceID,
		ProjectName: projectName,
		DisplayName: displayName,
		SubmittedBy: submittedBy,
	}
}

func registerProjectReviewSubmitted(registry *events.Registry) error {
	return events.RegisterJSON[ProjectReviewSubmitted](
		registry,
		ProjectReviewSubmittedName,
		1,
		func(ev ProjectReviewSubmitted) events.AggregateRef {
			return events.AggregateRef{Type: "project", ID: ev.ProjectID.String()}
		},
	)
}
