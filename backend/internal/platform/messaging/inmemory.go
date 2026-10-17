package messaging

import (
	"context"
	"sync"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type inMemoryBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func NewInMemoryBus() Bus {
	return &inMemoryBus{handlers: make(map[string][]Handler)}
}

func (b *inMemoryBus) Subscribe(eventName string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], h)
}

func (b *inMemoryBus) Publish(ctx context.Context, ev ...events.Event) error {
	for _, e := range ev {
		b.mu.RLock()
		hs := append([]Handler(nil), b.handlers[e.EventName()]...)
		b.mu.RUnlock()
		for _, h := range hs {
			if err := h(ctx, e); err != nil {
				return err
			}
		}
	}
	return nil
}
