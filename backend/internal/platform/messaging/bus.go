package messaging

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type Handler func(ctx context.Context, event events.Event) error

// Bus publishes domain events to in-process subscribers.
type Bus interface {
	Publish(ctx context.Context, ev ...events.Event) error
	Subscribe(eventName, subscriber string, h Handler) error
}

type Subscription struct {
	EventName  string
	Version    int
	Subscriber string
	Handler    Handler
}

type Lifecycle interface {
	Start(ctx context.Context) error
	Close() error
}

// Queue publishes async job messages to external brokers (AMQP).
type Queue interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	Subscribe(ctx context.Context, topic string, h func(ctx context.Context, payload []byte) error) error
	Close() error
}
