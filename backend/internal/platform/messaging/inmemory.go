package messaging

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type namedHandler struct {
	subscriber string
	handler    Handler
}

type inMemoryBus struct {
	mu       sync.RWMutex
	registry *events.Registry
	handlers map[string][]namedHandler
}

func NewInMemoryBus(registries ...*events.Registry) Bus {
	var registry *events.Registry
	if len(registries) > 0 {
		registry = registries[0]
	}
	return &inMemoryBus{
		registry: registry,
		handlers: make(map[string][]namedHandler),
	}
}

func (b *inMemoryBus) Subscribe(eventName, subscriber string, h Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, registered := range b.handlers[eventName] {
		if registered.subscriber == subscriber {
			return fmt.Errorf("duplicate subscription for event %q and subscriber %q", eventName, subscriber)
		}
	}
	b.handlers[eventName] = append(b.handlers[eventName], namedHandler{
		subscriber: subscriber,
		handler:    h,
	})
	return nil
}

func (b *inMemoryBus) Publish(ctx context.Context, ev ...events.Event) error {
	for _, e := range ev {
		eventID := uuid.NewString()
		version := 1
		correlationID := eventID
		if b.registry != nil {
			envelope, err := b.registry.Encode(ctx, e)
			if err != nil {
				return err
			}
			eventID = envelope.ID
			version = envelope.Version
			correlationID = envelope.CorrelationID
		} else if parent, ok := events.MetadataFromContext(ctx); ok {
			if parent.CorrelationID != "" {
				correlationID = parent.CorrelationID
			} else if parent.EventID != "" {
				correlationID = parent.EventID
			}
		}
		b.mu.RLock()
		handlers := append([]namedHandler(nil), b.handlers[e.EventName()]...)
		b.mu.RUnlock()
		for _, registered := range handlers {
			handlerCtx := events.WithMetadata(ctx, events.Metadata{
				EventID:       eventID,
				EventType:     e.EventName(),
				EventVersion:  version,
				Subscriber:    registered.subscriber,
				CorrelationID: correlationID,
			})
			if err := registered.handler(handlerCtx, e); err != nil {
				return err
			}
		}
	}
	return nil
}
