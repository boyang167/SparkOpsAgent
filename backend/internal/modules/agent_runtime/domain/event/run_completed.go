package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const RunCompletedName = "agent_runtime.RunCompleted"

type RunCompleted struct {
	events.BaseEvent
	RunID          kernel.RunID
	ConversationID kernel.ConversationID
	AgentID        kernel.AgentID
	ProjectID      kernel.ProjectID
	MemberID       kernel.MemberID
	WorkspaceID    kernel.WorkspaceID
}

func NewRunCompleted(
	runID kernel.RunID,
	conversationID kernel.ConversationID,
	agentID kernel.AgentID,
	projectID kernel.ProjectID,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
) RunCompleted {
	return RunCompleted{
		BaseEvent:      events.NewBaseEvent(RunCompletedName),
		RunID:          runID,
		ConversationID: conversationID,
		AgentID:        agentID,
		ProjectID:      projectID,
		MemberID:       memberID,
		WorkspaceID:    wsID,
	}
}

func Register(registry *events.Registry) error {
	return events.RegisterJSON[RunCompleted](
		registry,
		RunCompletedName,
		1,
		func(ev RunCompleted) events.AggregateRef {
			return events.AggregateRef{Type: "run", ID: ev.RunID.String()}
		},
	)
}
