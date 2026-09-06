package integration

import (
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const EventSecretUpdated = "integration.SecretUpdated"

type SecretUpdated struct {
	events.BaseEvent
	WorkspaceID kernel.WorkspaceID
	SecretID    kernel.SecretID
	ServiceType string
}

func NewSecretUpdated(wsID kernel.WorkspaceID, secretID kernel.SecretID, serviceType string) SecretUpdated {
	return SecretUpdated{
		BaseEvent:   events.NewBaseEvent(EventSecretUpdated),
		WorkspaceID: wsID,
		SecretID:    secretID,
		ServiceType: serviceType,
	}
}
