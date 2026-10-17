package eventbus

import (
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type Handler = messaging.Handler

type Bus = messaging.Bus

func NewInMemory() Bus {
	return messaging.NewInMemoryBus()
}

// Event is re-exported for handlers that reference events.Event via eventbus package.
type Event = events.Event
