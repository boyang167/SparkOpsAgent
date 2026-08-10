package messaging

import (
	"context"
	"fmt"
	"sync"
)

// noopQueue is an in-memory queue for local dev when AMQP is unavailable.
type noopQueue struct {
	mu       sync.Mutex
	handlers map[string]func(context.Context, []byte) error
}

func NewNoopQueue() Queue {
	return &noopQueue{handlers: make(map[string]func(context.Context, []byte) error)}
}

func (q *noopQueue) Publish(ctx context.Context, topic string, payload []byte) error {
	q.mu.Lock()
	h := q.handlers[topic]
	q.mu.Unlock()
	if h == nil {
		return nil
	}
	return h(ctx, payload)
}

func (q *noopQueue) Subscribe(ctx context.Context, topic string, h func(context.Context, []byte) error) error {
	q.mu.Lock()
	q.handlers[topic] = h
	q.mu.Unlock()
	return nil
}

func (q *noopQueue) Close() error { return nil }

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
	switch topic {
	case TopicBuild, TopicCronJob, TopicAlarm, TopicNotice, TopicProclaim, TopicHook:
		return nil
	default:
		return fmt.Errorf("unknown queue topic: %s", topic)
	}
}
