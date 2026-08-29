package messaging

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AMQPConfig struct {
	URL          string
	Exchange     string
	ExchangeType string
}

type amqpQueue struct {
	cfg              AMQPConfig
	conn             *amqp.Connection
	ch               *amqp.Channel
	mu               sync.Mutex
	consumerChannels map[*amqp.Channel]struct{}
	closed           bool
}

type queueBinding struct {
	Name       string
	RoutingKey string
	Durable    bool
}

type deliveryPublisher func(context.Context, string, []byte, amqp.Table) error

type topologyChannel interface {
	QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error)
	QueueBind(string, string, string, bool, amqp.Table) error
}

type consumerResource interface {
	Cancel(string, bool) error
	Close() error
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
	return &amqpQueue{
		cfg: cfg, conn: conn, ch: ch,
		consumerChannels: make(map[*amqp.Channel]struct{}),
	}, nil
}

func (q *amqpQueue) Publish(ctx context.Context, topic string, payload []byte) error {
	return q.publishWithHeaders(ctx, topic, payload, nil)
}

func (q *amqpQueue) publishWithHeaders(ctx context.Context, topic string, payload []byte, headers amqp.Table) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.ch == nil {
		return fmt.Errorf("amqp channel is closed")
	}
	if err := ensureTopology(q.ch, q.cfg.Exchange, topic); err != nil {
		return err
	}
	return q.ch.PublishWithContext(ctx, q.cfg.Exchange, topic, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         payload,
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
	})
}

func (q *amqpQueue) Subscribe(ctx context.Context, topic string, h func(context.Context, []byte) error) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}
	q.mu.Lock()
	if q.closed || q.conn == nil {
		q.mu.Unlock()
		return fmt.Errorf("amqp connection is closed")
	}
	consumerChannel, err := q.conn.Channel()
	if err == nil {
		q.consumerChannels[consumerChannel] = struct{}{}
	}
	q.mu.Unlock()
	if err != nil {
		return fmt.Errorf("amqp consumer channel: %w", err)
	}
	if err := ensureTopology(consumerChannel, q.cfg.Exchange, topic); err != nil {
		_ = q.stopConsumer(consumerChannel, "")
		return err
	}
	consumerTag := "sparkops-" + strings.ReplaceAll(topic, ".", "-")
	deliveries, err := consumerChannel.Consume(topic, consumerTag, false, false, false, false, nil)
	if err != nil {
		_ = q.stopConsumer(consumerChannel, consumerTag)
		return fmt.Errorf("amqp consume: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				if closeErr := q.stopConsumer(consumerChannel, consumerTag); closeErr != nil {
					log.Printf("amqp consumer %s shutdown failed: %v", topic, closeErr)
				}
				return
			case d, ok := <-deliveries:
				if !ok {
					_ = q.detachConsumer(consumerChannel)
					return
				}
				retry := retryFromHeaders(d.Headers)
				publish := func(ctx context.Context, publishTopic string, body []byte, headers amqp.Table) error {
					if err := ensureTopology(consumerChannel, q.cfg.Exchange, topic); err != nil {
						return err
					}
					return consumerChannel.PublishWithContext(ctx, q.cfg.Exchange, publishTopic, false, false, amqp.Publishing{
						ContentType: "application/json", Body: body, DeliveryMode: amqp.Persistent, Headers: headers,
					})
				}
				if err := handleDelivery(ctx, topic, retry, d.Body, h, publish,
					func() error { return d.Ack(false) }); err != nil {
					log.Printf("amqp consumer %s stopped after delivery failure: %v", topic, err)
					if closeErr := q.stopConsumer(consumerChannel, consumerTag); closeErr != nil {
						log.Printf("amqp consumer %s shutdown failed: %v", topic, closeErr)
					}
					return
				}
			}
		}
	}()
	return nil
}

func consumeTopology(topic string) []queueBinding {
	deadLetter := DeadLetterTopic(topic)
	return []queueBinding{
		{Name: topic, RoutingKey: topic, Durable: true},
		{Name: deadLetter, RoutingKey: deadLetter, Durable: true},
	}
}

func ensureTopology(channel topologyChannel, exchange, topic string) error {
	for _, binding := range consumeTopology(topic) {
		if _, err := channel.QueueDeclare(binding.Name, binding.Durable, false, false, false, nil); err != nil {
			return fmt.Errorf("amqp queue declare %s: %w", binding.Name, err)
		}
		if err := channel.QueueBind(binding.Name, binding.RoutingKey, exchange, false, nil); err != nil {
			return fmt.Errorf("amqp queue bind %s: %w", binding.Name, err)
		}
	}
	return nil
}

func handleDelivery(
	ctx context.Context,
	topic string,
	retry int,
	body []byte,
	handler func(context.Context, []byte) error,
	publish deliveryPublisher,
	ack func() error,
) error {
	decision := DecideConsume(topic, retry, handler(ctx, body))
	if decision.RepublishTopic != "" {
		headers := amqp.Table{HeaderRetry: int32(decision.RepublishRetry)}
		if err := publish(ctx, decision.RepublishTopic, body, headers); err != nil {
			return fmt.Errorf("amqp republish delivery: %w", err)
		}
	}
	if decision.DeadLetterTopic != "" {
		headers := amqp.Table{HeaderRetry: int32(retry + 1)}
		if err := publish(ctx, decision.DeadLetterTopic, body, headers); err != nil {
			return fmt.Errorf("amqp publish dead letter: %w", err)
		}
	}
	if decision.Ack {
		if err := ack(); err != nil {
			return fmt.Errorf("amqp ack delivery: %w", err)
		}
	}
	return nil
}

func (q *amqpQueue) stopConsumer(channel *amqp.Channel, consumerTag string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.consumerChannels[channel]; !exists {
		return nil
	}
	delete(q.consumerChannels, channel)
	return stopConsumerChannel(channel, consumerTag)
}

func (q *amqpQueue) detachConsumer(channel *amqp.Channel) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.consumerChannels[channel]; !exists {
		return false
	}
	delete(q.consumerChannels, channel)
	return true
}

func stopConsumerChannel(channel consumerResource, consumerTag string) error {
	var errs []error
	if consumerTag != "" {
		if err := channel.Cancel(consumerTag, false); err != nil {
			errs = append(errs, fmt.Errorf("cancel consumer: %w", err))
		}
	}
	if err := channel.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close channel: %w", err))
	}
	return errors.Join(errs...)
}

func retryFromHeaders(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	switch value := headers[HeaderRetry].(type) {
	case int32:
		return int(value)
	case int64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func (q *amqpQueue) Close() error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	publishChannel := q.ch
	connection := q.conn
	consumerChannels := make([]*amqp.Channel, 0, len(q.consumerChannels))
	for channel := range q.consumerChannels {
		consumerChannels = append(consumerChannels, channel)
	}
	q.ch = nil
	q.conn = nil
	q.consumerChannels = make(map[*amqp.Channel]struct{})
	q.mu.Unlock()

	var errs []error
	for _, channel := range consumerChannels {
		if err := channel.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close consumer channel: %w", err))
		}
	}
	if publishChannel != nil {
		if err := publishChannel.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close publish channel: %w", err))
		}
	}
	if connection != nil {
		if err := connection.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close connection: %w", err))
		}
	}
	return errors.Join(errs...)
}
