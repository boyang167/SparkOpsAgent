package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type InboxConsumer struct {
	transactor  postgres.Transactor
	inbox       persistence.InboxStore
	registry    *events.Registry
	transport   EventTransport
	clock       Clock
	metrics     *Metrics
	lifecycleMu sync.Mutex
	stopping    bool
	active      sync.WaitGroup
}

func NewInboxConsumer(
	transactor postgres.Transactor,
	inbox persistence.InboxStore,
	registry *events.Registry,
	transport EventTransport,
	clock Clock,
	metrics *Metrics,
) *InboxConsumer {
	if clock == nil {
		clock = systemClock{}
	}
	return &InboxConsumer{
		transactor: transactor,
		inbox:      inbox,
		registry:   registry,
		transport:  transport,
		clock:      clock,
		metrics:    metrics,
	}
}

func (c *InboxConsumer) Handle(
	ctx context.Context,
	subscription Subscription,
	envelope events.Envelope,
	attempts int,
) error {
	if c == nil || c.transactor == nil || c.inbox == nil || c.registry == nil || c.transport == nil {
		return fmt.Errorf("inbox consumer dependencies are required")
	}
	c.lifecycleMu.Lock()
	if c.stopping {
		c.lifecycleMu.Unlock()
		return context.Canceled
	}
	c.active.Add(1)
	c.lifecycleMu.Unlock()
	defer c.active.Done()

	duplicate := false
	err := c.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		inserted, err := c.inbox.TryInsert(
			txCtx,
			envelope.ID,
			subscription.Subscriber,
			envelope.Type,
			c.clock.Now(),
		)
		if err != nil {
			return err
		}
		if !inserted {
			duplicate = true
			return nil
		}
		event, err := c.registry.Decode(envelope)
		if err != nil {
			return Permanent(err)
		}
		handlerCtx := events.WithMetadata(txCtx, events.Metadata{
			EventID:       envelope.ID,
			EventType:     envelope.Type,
			EventVersion:  envelope.Version,
			Subscriber:    subscription.Subscriber,
			CorrelationID: envelope.CorrelationID,
		})
		return subscription.Handler(handlerCtx, event)
	})
	if err == nil {
		if c.metrics != nil {
			result := "success"
			if duplicate {
				result = "duplicate"
				c.metrics.InboxDuplicate.WithLabelValues(subscription.Subscriber).Inc()
			}
			c.metrics.EventConsume.WithLabelValues(subscription.Subscriber, result).Inc()
		}
		return nil
	}
	if ctx.Err() != nil {
		return err
	}
	if IsPermanent(err) {
		if publishErr := c.transport.PublishDeadLetter(ctx, subscription, envelope, err.Error()); publishErr != nil {
			if c.metrics != nil {
				c.metrics.EventConsume.WithLabelValues(
					subscription.Subscriber,
					"dlq_publish_failure",
				).Inc()
			}
			return fmt.Errorf("publish event dead letter: %w", publishErr)
		}
		if c.metrics != nil {
			c.metrics.EventDLQ.WithLabelValues(subscription.Subscriber).Inc()
			c.metrics.EventConsume.WithLabelValues(subscription.Subscriber, "permanent_error").Inc()
		}
		return nil
	}

	delay := RetryDelay(retryDelay(attempts + 1))
	if publishErr := c.transport.PublishRetry(ctx, subscription, envelope, delay, attempts+1); publishErr != nil {
		if c.metrics != nil {
			c.metrics.EventConsume.WithLabelValues(
				subscription.Subscriber,
				"retry_publish_failure",
			).Inc()
		}
		return fmt.Errorf("publish event retry: %w", publishErr)
	}
	if c.metrics != nil {
		c.metrics.EventRetry.WithLabelValues(subscription.Subscriber, retryDelayLabel(delay)).Inc()
		c.metrics.EventConsume.WithLabelValues(subscription.Subscriber, "retry").Inc()
	}
	return nil
}

func (c *InboxConsumer) StopAccepting() {
	if c == nil {
		return
	}
	c.lifecycleMu.Lock()
	c.stopping = true
	c.lifecycleMu.Unlock()
}

func (c *InboxConsumer) Wait() {
	if c == nil {
		return
	}
	c.active.Wait()
}

func (c *InboxConsumer) StopAndWait() {
	c.StopAccepting()
	c.Wait()
}

func retryDelayLabel(delay RetryDelay) string {
	switch delay {
	case RetryDelay(5 * time.Second):
		return "5s"
	case RetryDelay(30 * time.Second):
		return "30s"
	case RetryDelay(5 * time.Minute):
		return "5m"
	default:
		return "30m"
	}
}
