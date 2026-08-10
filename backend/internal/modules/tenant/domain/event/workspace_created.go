package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const WorkspaceCreatedName = "tenant.WorkspaceCreated"

type WorkspaceCreated struct {
	events.BaseEvent
	WorkspaceID kernel.WorkspaceID
	OrgID       kernel.OrgID
	OwnerID     kernel.MemberID
}

func NewWorkspaceCreated(wsID kernel.WorkspaceID, orgID kernel.OrgID, ownerID kernel.MemberID) WorkspaceCreated {
	return WorkspaceCreated{
		BaseEvent:   events.NewBaseEvent(WorkspaceCreatedName),
		WorkspaceID: wsID,
		OrgID:       orgID,
		OwnerID:     ownerID,
	}
}
