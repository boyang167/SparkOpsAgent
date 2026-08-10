package messaging

import (
	"context"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AMQPConfig struct {
	URL          string
	Exchange     string
	ExchangeType string
}

type amqpQueue struct {
	cfg  AMQPConfig
	conn *amqp.Connection
	ch   *amqp.Channel
	mu   sync.Mutex
}

func NewAMQPQueue(cfg AMQPConfig) (Queue, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("amqp url is required")
	}
	if cfg.Exchange == "" {
		cfg.Exchange = "sparkops"
	}
	if cfg.ExchangeType == "" {
		cfg.ExchangeType = "direct"
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("amqp dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("amqp channel: %w", err)
	}
	if err := ch.ExchangeDeclare(cfg.Exchange, cfg.ExchangeType, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("amqp exchange declare: %w", err)
	}
	return &amqpQueue{cfg: cfg, conn: conn, ch: ch}, nil
}

func (q *amqpQueue) Publish(ctx context.Context, topic string, payload []byte) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}
	_ = ctx
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.ch.PublishWithContext(ctx, q.cfg.Exchange, topic, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         payload,
		DeliveryMode: amqp.Persistent,
	})
}

func (q *amqpQueue) Subscribe(ctx context.Context, topic string, h func(context.Context, []byte) error) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, err := q.ch.QueueDeclare(topic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("amqp queue declare: %w", err)
	}
	if err := q.ch.QueueBind(topic, topic, q.cfg.Exchange, false, nil); err != nil {
		return fmt.Errorf("amqp queue bind: %w", err)
	}
	deliveries, err := q.ch.Consume(topic, "", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("amqp consume: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				_ = h(ctx, d.Body)
			}
		}
	}()
	return nil
}

func (q *amqpQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ch != nil {
		_ = q.ch.Close()
	}
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}
