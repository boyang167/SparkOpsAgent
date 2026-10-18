package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"go.uber.org/zap"
)

type InboxConsumer struct {
	transactor  postgres.Transactor
	inbox       persistence.InboxStore
	registry    *events.Registry
	transport   EventTransport
	clock       Clock
	metrics     *Metrics
	logger      *zap.Logger
	lifecycleMu sync.Mutex
	stopping    bool
	active      sync.WaitGroup
}

type inboxConsumerContextKey struct{}

const maxEventRetryAttempts = 4

func IsInboxConsumerContext(ctx context.Context) bool {
	inbox, _ := ctx.Value(inboxConsumerContextKey{}).(bool)
	return inbox
}

func NewInboxConsumer(
	transactor postgres.Transactor,
	inbox persistence.InboxStore,
	registry *events.Registry,
	transport EventTransport,
	clock Clock,
	metrics *Metrics,
	loggers ...*zap.Logger,
) *InboxConsumer {
	if clock == nil {
		clock = systemClock{}
	}
	logger := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return &InboxConsumer{
		transactor: transactor,
		inbox:      inbox,
		registry:   registry,
		transport:  transport,
		clock:      clock,
		metrics:    metrics,
		logger:     logger,
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
		handlerCtx = context.WithValue(handlerCtx, inboxConsumerContextKey{}, true)
		return subscription.Handler(handlerCtx, event)
	})
	if err == nil {
		c.logger.Debug(
			"event consumed",
			zap.String("event_id", envelope.ID),
			zap.String("event_type", envelope.Type),
			zap.String("subscriber", subscription.Subscriber),
			zap.Int("attempt", attempts+1),
			zap.Bool("duplicate", duplicate),
		)
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
		c.logger.Error(
			"event routed to dead letter queue",
			zap.String("event_id", envelope.ID),
			zap.String("event_type", envelope.Type),
			zap.String("subscriber", subscription.Subscriber),
			zap.Int("attempt", attempts+1),
			zap.Error(err),
		)
		if publishErr := c.transport.PublishDeadLetter(
			ctx, subscription, envelope, err.Error(), attempts,
		); publishErr != nil {
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
	if attempts >= maxEventRetryAttempts {
		reason := fmt.Sprintf("retry attempts exhausted after %d retries: %v", attempts, err)
		c.logger.Error(
			"event retries exhausted; routed to dead letter queue",
			zap.String("event_id", envelope.ID),
			zap.String("event_type", envelope.Type),
			zap.String("subscriber", subscription.Subscriber),
			zap.Int("attempt", attempts+1),
			zap.Error(err),
		)
		if publishErr := c.transport.PublishDeadLetter(
			ctx, subscription, envelope, reason, attempts,
		); publishErr != nil {
			if c.metrics != nil {
				c.metrics.EventConsume.WithLabelValues(
					subscription.Subscriber,
					"dlq_publish_failure",
				).Inc()
			}
			return fmt.Errorf("publish exhausted event dead letter: %w", publishErr)
		}
		if c.metrics != nil {
			c.metrics.EventDLQ.WithLabelValues(subscription.Subscriber).Inc()
			c.metrics.EventConsume.WithLabelValues(subscription.Subscriber, "retry_exhausted").Inc()
		}
		return nil
	}

	delay := RetryDelay(retryDelay(attempts + 1))
	c.logger.Warn(
		"event scheduled for retry",
		zap.String("event_id", envelope.ID),
		zap.String("event_type", envelope.Type),
		zap.String("subscriber", subscription.Subscriber),
		zap.Int("attempt", attempts+1),
		zap.Duration("delay", time.Duration(delay)),
		zap.Error(err),
	)
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
