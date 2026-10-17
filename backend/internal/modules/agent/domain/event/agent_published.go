package event

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const AgentPublishedName = "agent.AgentPublished"

type AgentPublished struct {
	events.BaseEvent
	AgentID   kernel.AgentID
	ProjectID kernel.ProjectID
	VersionID kernel.AgentVersionID
	Version   int
}

func NewAgentPublished(agentID kernel.AgentID, projectID kernel.ProjectID, versionID kernel.AgentVersionID, version int) AgentPublished {
	return AgentPublished{
		BaseEvent: events.NewBaseEvent(AgentPublishedName),
		AgentID:   agentID,
		ProjectID: projectID,
		VersionID: versionID,
		Version:   version,
	}
}
