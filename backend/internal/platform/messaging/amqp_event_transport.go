package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

const eventAttemptsHeader = "x-sparkops-attempts"

type RetryDelay time.Duration

type DeliveryHandler func(ctx context.Context, envelope events.Envelope, attempts int) error

type EventTransport interface {
	Declare(ctx context.Context, subscriptions []Subscription) error
	Publish(ctx context.Context, envelope events.Envelope) error
	Consume(ctx context.Context, subscription Subscription, handler DeliveryHandler) error
	PublishRetry(ctx context.Context, subscription Subscription, envelope events.Envelope, delay RetryDelay, attempts int) error
	PublishDeadLetter(ctx context.Context, subscription Subscription, envelope events.Envelope, reason string) error
	Close() error
}

type eventConfirmation interface {
	Done() <-chan struct{}
	Acked() bool
}

type eventPublishChannel interface {
	PublishWithDeferredConfirmWithContext(
		context.Context,
		string,
		string,
		bool,
		bool,
		amqp.Publishing,
	) (eventConfirmation, error)
}

type eventChannel interface {
	eventPublishChannel
	Confirm(bool) error
	ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error
	QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error)
	QueueBind(string, string, string, bool, amqp.Table) error
	Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error)
	Cancel(string, bool) error
	NotifyReturn(chan amqp.Return) chan amqp.Return
	NotifyClose(chan *amqp.Error) chan *amqp.Error
	Close() error
}

type eventConnection interface {
	Channel() (eventChannel, error)
	Close() error
}

type realEventConnection struct {
	connection *amqp.Connection
}

func (c *realEventConnection) Channel() (eventChannel, error) {
	channel, err := c.connection.Channel()
	if err != nil {
		return nil, err
	}
	return &realEventChannel{channel: channel}, nil
}

func (c *realEventConnection) Close() error {
	return c.connection.Close()
}

type realEventChannel struct {
	channel *amqp.Channel
}

func (c *realEventChannel) PublishWithDeferredConfirmWithContext(
	ctx context.Context,
	exchange string,
	key string,
	mandatory bool,
	immediate bool,
	message amqp.Publishing,
) (eventConfirmation, error) {
	return c.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		exchange,
		key,
		mandatory,
		immediate,
		message,
	)
}

func (c *realEventChannel) Confirm(noWait bool) error {
	return c.channel.Confirm(noWait)
}

func (c *realEventChannel) ExchangeDeclare(
	name string,
	kind string,
	durable bool,
	autoDelete bool,
	internal bool,
	noWait bool,
	args amqp.Table,
) error {
	return c.channel.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (c *realEventChannel) QueueDeclare(
	name string,
	durable bool,
	autoDelete bool,
	exclusive bool,
	noWait bool,
	args amqp.Table,
) (amqp.Queue, error) {
	return c.channel.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

func (c *realEventChannel) QueueBind(
	name string,
	key string,
	exchange string,
	noWait bool,
	args amqp.Table,
) error {
	return c.channel.QueueBind(name, key, exchange, noWait, args)
}

func (c *realEventChannel) Consume(
	queue string,
	consumer string,
	autoAck bool,
	exclusive bool,
	noLocal bool,
	noWait bool,
	args amqp.Table,
) (<-chan amqp.Delivery, error) {
	return c.channel.Consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
}

func (c *realEventChannel) Cancel(consumer string, noWait bool) error {
	return c.channel.Cancel(consumer, noWait)
}

func (c *realEventChannel) NotifyReturn(receiver chan amqp.Return) chan amqp.Return {
	return c.channel.NotifyReturn(receiver)
}

func (c *realEventChannel) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	return c.channel.NotifyClose(receiver)
}

func (c *realEventChannel) Close() error {
	return c.channel.Close()
}

type amqpEventTransport struct {
	mu               sync.Mutex
	publishMu        sync.Mutex
	connection       eventConnection
	publisher        eventChannel
	publisherReturns <-chan amqp.Return
	publisherClosed  <-chan *amqp.Error
	consumers        map[eventChannel]string
	closed           bool
}

func NewAMQPEventTransport(url string) (EventTransport, error) {
	if url == "" {
		return nil, fmt.Errorf("amqp url is required")
	}
	connection, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("amqp event dial: %w", err)
	}
	return newAMQPEventTransport(&realEventConnection{connection: connection}), nil
}

func newAMQPEventTransport(connection eventConnection) *amqpEventTransport {
	return &amqpEventTransport{
		connection: connection,
		consumers:  make(map[eventChannel]string),
	}
}

func (t *amqpEventTransport) Declare(ctx context.Context, subscriptions []Subscription) error {
	if err := validateEventTopologies(subscriptions); err != nil {
		return err
	}

	t.publishMu.Lock()
	defer t.publishMu.Unlock()
	t.mu.Lock()
	if err := ctx.Err(); err != nil {
		t.mu.Unlock()
		return err
	}
	channel, err := t.publisherChannelLocked()
	if err != nil {
		t.mu.Unlock()
		return err
	}
	t.mu.Unlock()
	if err := channel.ExchangeDeclare(eventExchangeName, "topic", true, false, false, false, nil); err != nil {
		t.invalidatePublisher(channel)
		return fmt.Errorf("declare event exchange: %w", err)
	}
	for _, subscription := range subscriptions {
		if err := declareEventTopology(ctx, channel, topologyFor(subscription)); err != nil {
			t.invalidatePublisher(channel)
			return err
		}
	}
	return nil
}

func declareEventTopology(ctx context.Context, channel eventChannel, topology eventTopology) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := channel.QueueDeclare(topology.Main.Name, true, false, false, false, topology.Main.Args); err != nil {
		return fmt.Errorf("declare event main queue %s: %w", topology.Main.Name, err)
	}
	if err := channel.QueueBind(
		topology.Main.Name,
		topology.Main.RoutingKey,
		eventExchangeName,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind event main queue %s: %w", topology.Main.Name, err)
	}
	for _, queue := range topology.Retry {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := channel.QueueDeclare(queue.Name, true, false, false, false, queue.Args); err != nil {
			return fmt.Errorf("declare event retry queue %s: %w", queue.Name, err)
		}
	}
	if _, err := channel.QueueDeclare(
		topology.DeadLetter.Name,
		true,
		false,
		false,
		false,
		topology.DeadLetter.Args,
	); err != nil {
		return fmt.Errorf("declare event dead-letter queue %s: %w", topology.DeadLetter.Name, err)
	}
	return nil
}

func (t *amqpEventTransport) Publish(ctx context.Context, envelope events.Envelope) error {
	routingKey := fmt.Sprintf("%s.v%d", envelope.Type, envelope.Version)
	return t.publish(ctx, eventExchangeName, routingKey, envelope, nil)
}

func (t *amqpEventTransport) PublishRetry(
	ctx context.Context,
	subscription Subscription,
	envelope events.Envelope,
	delay RetryDelay,
	attempts int,
) error {
	topology := topologyFor(subscription)
	index, err := retryTierIndex(delay)
	if err != nil {
		return err
	}
	return t.publish(
		ctx,
		"",
		topology.Retry[index].Name,
		envelope,
		amqp.Table{eventAttemptsHeader: int32(attempts)},
	)
}

func retryTierIndex(delay RetryDelay) (int, error) {
	switch time.Duration(delay) {
	case 5 * time.Second:
		return 0, nil
	case 30 * time.Second:
		return 1, nil
	case 5 * time.Minute:
		return 2, nil
	case 30 * time.Minute:
		return 3, nil
	default:
		return 0, fmt.Errorf("unsupported event retry delay %s", time.Duration(delay))
	}
}

func (t *amqpEventTransport) PublishDeadLetter(
	ctx context.Context,
	subscription Subscription,
	envelope events.Envelope,
	reason string,
) error {
	return t.publish(
		ctx,
		"",
		topologyFor(subscription).DeadLetter.Name,
		envelope,
		amqp.Table{"x-sparkops-dead-letter-reason": reason},
	)
}

func (t *amqpEventTransport) publish(
	ctx context.Context,
	exchange string,
	routingKey string,
	envelope events.Envelope,
	headers amqp.Table,
) error {
	t.publishMu.Lock()
	defer t.publishMu.Unlock()
	t.mu.Lock()
	channel, err := t.publisherChannelLocked()
	if err != nil {
		t.mu.Unlock()
		return err
	}
	returns := t.publisherReturns
	closed := t.publisherClosed
	t.mu.Unlock()
	err = publishEventConfirmed(
		ctx,
		channel,
		returns,
		closed,
		exchange,
		routingKey,
		envelope,
		headers,
	)
	if err != nil {
		t.invalidatePublisher(channel)
	}
	return err
}

func publishEventConfirmed(
	ctx context.Context,
	channel eventPublishChannel,
	returns <-chan amqp.Return,
	closed <-chan *amqp.Error,
	exchange string,
	routingKey string,
	envelope events.Envelope,
	headers amqp.Table,
) error {
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal event envelope: %w", err)
	}
	confirmation, err := channel.PublishWithDeferredConfirmWithContext(
		ctx,
		exchange,
		routingKey,
		true,
		false,
		amqp.Publishing{
			Headers:      headers,
			MessageId:    envelope.ID,
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    envelope.OccurredAt,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	if confirmation == nil {
		return fmt.Errorf("publish event: confirm mode is not enabled")
	}

	select {
	case returned, ok := <-returns:
		if !ok {
			return fmt.Errorf("publish event: return listener closed")
		}
		return fmt.Errorf(
			"publish event returned: code=%d text=%s",
			returned.ReplyCode,
			returned.ReplyText,
		)
	case closeErr, ok := <-closed:
		if !ok || closeErr == nil {
			return fmt.Errorf("publish event: channel closed")
		}
		return fmt.Errorf("publish event: channel closed: %s", closeErr.Reason)
	case <-confirmation.Done():
		if !confirmation.Acked() {
			return fmt.Errorf("publish event: broker nack")
		}
		select {
		case returned := <-returns:
			return fmt.Errorf(
				"publish event returned: code=%d text=%s",
				returned.ReplyCode,
				returned.ReplyText,
			)
		default:
			return nil
		}
	case <-ctx.Done():
		return fmt.Errorf("publish event confirmation: %w", ctx.Err())
	}
}

func (t *amqpEventTransport) publisherChannelLocked() (eventChannel, error) {
	if t.closed || t.connection == nil {
		return nil, fmt.Errorf("amqp event transport is closed")
	}
	if t.publisher != nil {
		return t.publisher, nil
	}
	channel, err := t.connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("open event publisher channel: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		return nil, fmt.Errorf("enable event publisher confirms: %w", err)
	}
	t.publisher = channel
	t.publisherReturns = channel.NotifyReturn(make(chan amqp.Return, 1))
	t.publisherClosed = channel.NotifyClose(make(chan *amqp.Error, 1))
	return channel, nil
}

func (t *amqpEventTransport) invalidatePublisher(channel eventChannel) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.publisher != channel {
		return
	}
	t.invalidatePublisherLocked()
}

func (t *amqpEventTransport) invalidatePublisherLocked() {
	if t.publisher != nil {
		_ = t.publisher.Close()
	}
	t.publisher = nil
	t.publisherReturns = nil
	t.publisherClosed = nil
}

func (t *amqpEventTransport) Consume(
	ctx context.Context,
	subscription Subscription,
	handler DeliveryHandler,
) error {
	if handler == nil {
		return fmt.Errorf("event delivery handler is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	t.mu.Lock()
	if t.closed || t.connection == nil {
		t.mu.Unlock()
		return fmt.Errorf("amqp event transport is closed")
	}
	channel, err := t.connection.Channel()
	if err != nil {
		t.mu.Unlock()
		return fmt.Errorf("open event consumer channel: %w", err)
	}
	topology := topologyFor(subscription)
	consumerTag := "sparkops-event-" + normalizeQueueNamePart(subscription.Subscriber, false)
	deliveries, err := channel.Consume(topology.Main.Name, consumerTag, false, false, false, false, nil)
	if err != nil {
		t.mu.Unlock()
		_ = channel.Close()
		return fmt.Errorf("consume event queue %s: %w", topology.Main.Name, err)
	}
	t.consumers[channel] = consumerTag
	t.mu.Unlock()

	go t.consumeDeliveries(ctx, channel, consumerTag, deliveries, handler)
	return nil
}

func (t *amqpEventTransport) consumeDeliveries(
	ctx context.Context,
	channel eventChannel,
	consumerTag string,
	deliveries <-chan amqp.Delivery,
	handler DeliveryHandler,
) {
	defer t.stopConsumer(channel, consumerTag)
	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			var envelope events.Envelope
			if err := json.Unmarshal(delivery.Body, &envelope); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}
			attempts := eventDeliveryAttempts(delivery.Headers)
			if err := handler(ctx, envelope, attempts); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}
			_ = delivery.Ack(false)
		}
	}
}

func eventDeliveryAttempts(headers amqp.Table) int {
	switch value := headers[eventAttemptsHeader].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	default:
		return 0
	}
}

func (t *amqpEventTransport) stopConsumer(channel eventChannel, consumerTag string) {
	t.mu.Lock()
	if _, exists := t.consumers[channel]; !exists {
		t.mu.Unlock()
		return
	}
	delete(t.consumers, channel)
	t.mu.Unlock()
	_ = channel.Cancel(consumerTag, false)
	_ = channel.Close()
}

func (t *amqpEventTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	publisher := t.publisher
	connection := t.connection
	consumers := make([]eventChannel, 0, len(t.consumers))
	for channel := range t.consumers {
		consumers = append(consumers, channel)
	}
	t.publisher = nil
	t.connection = nil
	t.publisherReturns = nil
	t.publisherClosed = nil
	t.consumers = make(map[eventChannel]string)
	t.mu.Unlock()

	var closeErrors []error
	for _, channel := range consumers {
		if err := channel.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close event consumer channel: %w", err))
		}
	}
	if publisher != nil {
		if err := publisher.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close event publisher channel: %w", err))
		}
	}
	if connection != nil {
		if err := connection.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close event connection: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}
