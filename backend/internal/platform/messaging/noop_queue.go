package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// noopQueue is an in-memory queue for local dev when AMQP is unavailable.
type noopQueue struct {
	mu           sync.Mutex
	handlers     map[string]func(context.Context, []byte) error
	pending      map[string][]pendingMessage
	pendingCount int
	draining     map[string]bool
}

type pendingMessage struct {
	payload []byte
	retry   int
}

const NoopPendingCapacity = 1024

var ErrNoopQueueFull = errors.New("in-memory queue pending capacity reached")

func NewNoopQueue() Queue {
	return &noopQueue{
		handlers: make(map[string]func(context.Context, []byte) error),
		pending:  make(map[string][]pendingMessage),
		draining: make(map[string]bool),
	}
}

func (q *noopQueue) Publish(ctx context.Context, topic string, payload []byte) error {
	return q.publish(ctx, topic, payload, 0)
}

func (q *noopQueue) publish(ctx context.Context, topic string, payload []byte, retry int) error {
	q.mu.Lock()
	if err := q.enqueueLocked(topic, payload, retry); err != nil {
		q.mu.Unlock()
		return err
	}
	if q.handlers[topic] == nil || q.draining[topic] {
		q.mu.Unlock()
		return nil
	}
	q.draining[topic] = true
	q.mu.Unlock()
	return q.drain(ctx, topic)
}

func (q *noopQueue) Subscribe(ctx context.Context, topic string, h func(context.Context, []byte) error) error {
	q.mu.Lock()
	q.handlers[topic] = h
	if q.draining[topic] || len(q.pending[topic]) == 0 {
		q.mu.Unlock()
		return nil
	}
	q.draining[topic] = true
	q.mu.Unlock()
	return q.drain(ctx, topic)
}

func (q *noopQueue) enqueueLocked(topic string, payload []byte, retry int) error {
	if q.pendingCount >= NoopPendingCapacity {
		return ErrNoopQueueFull
	}
	q.pending[topic] = append(q.pending[topic], pendingMessage{
		payload: append([]byte(nil), payload...),
		retry:   retry,
	})
	q.pendingCount++
	return nil
}

func (q *noopQueue) drain(ctx context.Context, topic string) error {
	for {
		q.mu.Lock()
		pending := q.pending[topic]
		h := q.handlers[topic]
		if len(pending) == 0 || h == nil {
			q.draining[topic] = false
			q.mu.Unlock()
			return nil
		}
		message := pending[0]
		q.mu.Unlock()

		handlerErr := h(ctx, message.payload)
		decision := ConsumeDecision{Ack: true}
		if !strings.HasSuffix(topic, ".dlq") {
			decision = DecideConsume(topic, message.retry, handlerErr)
		}

		var nextTopic string
		q.mu.Lock()
		if decision.RepublishTopic != "" {
			if err := q.enqueueLocked(decision.RepublishTopic, message.payload, decision.RepublishRetry); err != nil {
				q.draining[topic] = false
				q.mu.Unlock()
				return err
			}
			nextTopic = decision.RepublishTopic
		} else if decision.DeadLetterTopic != "" {
			if err := q.enqueueLocked(decision.DeadLetterTopic, message.payload, message.retry+1); err != nil {
				q.draining[topic] = false
				q.mu.Unlock()
				return err
			}
			nextTopic = decision.DeadLetterTopic
		}
		pending = q.pending[topic]
		if len(pending) > 0 {
			q.pending[topic] = pending[1:]
			q.pendingCount--
			if len(q.pending[topic]) == 0 {
				delete(q.pending, topic)
			}
		}
		q.mu.Unlock()
		if nextTopic != "" && nextTopic != topic {
			q.mu.Lock()
			canDrain := q.handlers[nextTopic] != nil && !q.draining[nextTopic]
			if canDrain {
				q.draining[nextTopic] = true
			}
			q.mu.Unlock()
			if canDrain {
				if err := q.drain(ctx, nextTopic); err != nil {
					q.mu.Lock()
					q.draining[topic] = false
					q.mu.Unlock()
					return err
				}
			}
		}
	}
}

func (q *noopQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers = make(map[string]func(context.Context, []byte) error)
	q.pending = make(map[string][]pendingMessage)
	q.draining = make(map[string]bool)
	q.pendingCount = 0
	return nil
}

// Queue topic names aligned with legacy PlatformSuite queues (sparkops prefix).
const (
	TopicBuild    = "build.sparkops"
	TopicCronJob  = "cronjob.sparkops"
	TopicAlarm    = "alarm.sparkops"
	TopicNotice   = "notice.sparkops"
	TopicProclaim = "proclaim.sparkops"
	TopicHook     = "hook.sparkops"
)

func ValidateTopic(topic string) error {
	base := strings.TrimSuffix(topic, ".dlq")
	switch base {
	case TopicBuild, TopicCronJob, TopicAlarm, TopicNotice, TopicProclaim, TopicHook, TopicDeploy:
		return nil
	default:
		return fmt.Errorf("unknown queue topic: %s", topic)
	}
}
