package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

const (
	eventAttemptsHeader       = "x-sparkops-attempts"
	eventPublishAttemptHeader = "x-sparkops-publish-attempt"
	eventCloseTimeout         = time.Second
)

var (
	errEventPublisherClosed                = errors.New("event publisher channel closed")
	errEventPublisherConfirmationAbandoned = errors.New("event publisher confirmation abandoned")
)

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
	IsClosed() bool
	Close() error
}

type eventConnection interface {
	Channel() (eventChannel, error)
	CloseDeadline(time.Time) error
}

type eventConnectionFactory func() (eventConnection, error)

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

func (c *realEventConnection) CloseDeadline(deadline time.Time) error {
	return c.connection.CloseDeadline(deadline)
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

func (c *realEventChannel) IsClosed() bool {
	return c.channel.IsClosed()
}

func (c *realEventChannel) Close() error {
	return c.channel.Close()
}

type amqpEventTransport struct {
	mu                  sync.Mutex
	publishMu           sync.Mutex
	topologyMu          sync.Mutex
	publisherConnection eventConnection
	consumerConnection  eventConnection
	sharedConnection    bool
	connectionFactory   eventConnectionFactory
	publisher           eventChannel
	publisherReturns    <-chan amqp.Return
	publisherClosed     <-chan *amqp.Error
	topology            eventChannel
	consumers           map[eventChannel]string
	metrics             *Metrics
	closing             chan struct{}
	closed              bool
}

func NewAMQPEventTransport(url string, metrics *Metrics) (EventTransport, error) {
	if url == "" {
		return nil, fmt.Errorf("amqp url is required")
	}
	factory := func() (eventConnection, error) {
		connection, err := amqp.Dial(url)
		if err != nil {
			return nil, err
		}
		return &realEventConnection{connection: connection}, nil
	}
	publisherConnection, err := factory()
	if err != nil {
		return nil, fmt.Errorf("amqp event dial: %w", err)
	}
	consumerConnection, err := factory()
	if err != nil {
		_ = publisherConnection.CloseDeadline(time.Now().Add(eventCloseTimeout))
		return nil, fmt.Errorf("amqp event consumer dial: %w", err)
	}
	transport := newAMQPEventTransportWithConnections(
		publisherConnection,
		consumerConnection,
		factory,
	)
	transport.metrics = metrics
	return transport, nil
}

func newAMQPEventTransport(connection eventConnection) *amqpEventTransport {
	return newAMQPEventTransportWithConnections(connection, connection, nil)
}

func newAMQPEventTransportWithFactory(
	connection eventConnection,
	factory eventConnectionFactory,
) *amqpEventTransport {
	return newAMQPEventTransportWithConnections(connection, connection, factory)
}

func newAMQPEventTransportWithConnections(
	publisherConnection eventConnection,
	consumerConnection eventConnection,
	factory eventConnectionFactory,
) *amqpEventTransport {
	return &amqpEventTransport{
		publisherConnection: publisherConnection,
		consumerConnection:  consumerConnection,
		sharedConnection:    publisherConnection == consumerConnection,
		connectionFactory:   factory,
		consumers:           make(map[eventChannel]string),
		closing:             make(chan struct{}),
	}
}

func (t *amqpEventTransport) Declare(ctx context.Context, subscriptions []Subscription) error {
	if err := validateEventTopologies(subscriptions); err != nil {
		return err
	}

	t.topologyMu.Lock()
	defer t.topologyMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	channel, err := t.topologyChannel()
	if err != nil {
		return err
	}
	if err := channel.ExchangeDeclare(eventExchangeName, "topic", true, false, false, false, nil); err != nil {
		t.invalidateTopology(channel)
		return fmt.Errorf("declare event exchange: %w", err)
	}
	for _, subscription := range subscriptions {
		if err := declareEventTopology(ctx, channel, topologyFor(subscription)); err != nil {
			t.invalidateTopology(channel)
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

func (t *amqpEventTransport) publishRawDeadLetter(
	ctx context.Context,
	subscription Subscription,
	delivery amqp.Delivery,
	reason string,
) error {
	t.publishMu.Lock()
	defer t.publishMu.Unlock()
	channel, returns, closed, closing, err := t.publisherChannel()
	if err != nil {
		return err
	}
	headers := make(amqp.Table, len(delivery.Headers)+2)
	for key, value := range delivery.Headers {
		headers[key] = value
	}
	headers["x-sparkops-dead-letter-reason"] = reason
	headers = withPublishAttempt(headers)
	err = publishMessageConfirmed(
		ctx,
		channel,
		returns,
		closed,
		"",
		topologyFor(subscription).DeadLetter.Name,
		delivery.MessageId,
		headers,
		amqp.Publishing{
			Headers:         headers,
			ContentType:     delivery.ContentType,
			ContentEncoding: delivery.ContentEncoding,
			DeliveryMode:    amqp.Persistent,
			Priority:        delivery.Priority,
			CorrelationId:   delivery.CorrelationId,
			ReplyTo:         delivery.ReplyTo,
			Expiration:      delivery.Expiration,
			MessageId:       delivery.MessageId,
			Timestamp:       delivery.Timestamp,
			Type:            delivery.Type,
			UserId:          delivery.UserId,
			AppId:           delivery.AppId,
			Body:            append([]byte(nil), delivery.Body...),
		},
		closing,
	)
	if errors.Is(err, errEventPublisherConfirmationAbandoned) {
		if closeErr := t.retirePublisherConnection(channel); closeErr != nil {
			return errors.Join(err, closeErr)
		}
	} else if shouldInvalidatePublisher(err, channel) {
		t.invalidatePublisher(channel)
	}
	return err
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
	channel, returns, closed, closing, err := t.publisherChannel()
	if err != nil {
		return err
	}
	headers = withPublishAttempt(headers)
	err = publishEventConfirmed(
		ctx,
		channel,
		returns,
		closed,
		exchange,
		routingKey,
		envelope,
		headers,
		closing,
	)
	if errors.Is(err, errEventPublisherConfirmationAbandoned) {
		if closeErr := t.retirePublisherConnection(channel); closeErr != nil {
			return errors.Join(err, closeErr)
		}
	} else if shouldInvalidatePublisher(err, channel) {
		t.invalidatePublisher(channel)
	}
	return err
}

func withPublishAttempt(headers amqp.Table) amqp.Table {
	withAttempt := make(amqp.Table, len(headers)+1)
	for key, value := range headers {
		withAttempt[key] = value
	}
	withAttempt[eventPublishAttemptHeader] = uuid.NewString()
	return withAttempt
}

func shouldInvalidatePublisher(err error, channel eventChannel) bool {
	return errors.Is(err, errEventPublisherClosed) || channel.IsClosed()
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
	closing ...<-chan struct{},
) error {
	var stop <-chan struct{}
	if len(closing) > 0 {
		stop = closing[0]
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal event envelope: %w", err)
	}
	return publishMessageConfirmed(
		ctx,
		channel,
		returns,
		closed,
		exchange,
		routingKey,
		envelope.ID,
		headers,
		amqp.Publishing{
			Headers:      headers,
			MessageId:    envelope.ID,
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    envelope.OccurredAt,
			Body:         body,
		},
		stop,
	)
}

func publishMessageConfirmed(
	ctx context.Context,
	channel eventPublishChannel,
	returns <-chan amqp.Return,
	closed <-chan *amqp.Error,
	exchange string,
	routingKey string,
	messageID string,
	headers amqp.Table,
	message amqp.Publishing,
	stop <-chan struct{},
) error {
	confirmation, err := channel.PublishWithDeferredConfirmWithContext(
		ctx,
		exchange,
		routingKey,
		true,
		false,
		message,
	)
	if err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	if confirmation == nil {
		return fmt.Errorf("publish event: confirm mode is not enabled")
	}

	for {
		select {
		case returned, ok := <-returns:
			if !ok {
				return fmt.Errorf("publish event: return listener closed")
			}
			if !returnMatchesPublishAttempt(returned, messageID, headers) {
				continue
			}
			return fmt.Errorf(
				"publish event returned: code=%d text=%s",
				returned.ReplyCode,
				returned.ReplyText,
			)
		case closeErr, ok := <-closed:
			if !ok || closeErr == nil {
				return fmt.Errorf("%w: publish event: channel closed", errEventPublisherClosed)
			}
			return fmt.Errorf("%w: publish event: channel closed: %s", errEventPublisherClosed, closeErr.Reason)
		case <-confirmation.Done():
			if !confirmation.Acked() {
				return fmt.Errorf("publish event: broker nack")
			}
			for {
				select {
				case returned, ok := <-returns:
					if !ok {
						return fmt.Errorf("publish event: return listener closed")
					}
					if !returnMatchesPublishAttempt(returned, messageID, headers) {
						continue
					}
					return fmt.Errorf(
						"publish event returned: code=%d text=%s",
						returned.ReplyCode,
						returned.ReplyText,
					)
				default:
					return nil
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("%w: publish event confirmation: %w", errEventPublisherConfirmationAbandoned, ctx.Err())
		case <-stop:
			return fmt.Errorf("publish event: transport closed")
		}
	}
}

func returnMatchesPublishAttempt(returned amqp.Return, messageID string, headers amqp.Table) bool {
	if returned.MessageId != messageID {
		return false
	}
	expectedAttempt, expected := headers[eventPublishAttemptHeader]
	if !expected {
		_, returnedHasAttempt := returned.Headers[eventPublishAttemptHeader]
		return !returnedHasAttempt
	}
	returnedAttempt, returnedHasAttempt := returned.Headers[eventPublishAttemptHeader]
	return returnedHasAttempt && returnedAttempt == expectedAttempt
}

func (t *amqpEventTransport) publisherChannel() (
	eventChannel,
	<-chan amqp.Return,
	<-chan *amqp.Error,
	<-chan struct{},
	error,
) {
	for {
		t.mu.Lock()
		if t.closed {
			t.mu.Unlock()
			return nil, nil, nil, nil, fmt.Errorf("amqp event transport is closed")
		}
		if t.publisher != nil && !t.publisher.IsClosed() {
			channel := t.publisher
			returns := t.publisherReturns
			closed := t.publisherClosed
			closing := t.closing
			t.mu.Unlock()
			return channel, returns, closed, closing, nil
		}
		t.invalidatePublisherLocked()
		connection := t.publisherConnection
		factory := t.connectionFactory
		closing := t.closing
		t.mu.Unlock()

		if connection == nil {
			if factory == nil {
				return nil, nil, nil, nil, fmt.Errorf("amqp event transport connection is unavailable")
			}
			reconnected, err := factory()
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("reconnect event transport: %w", err)
			}
			t.mu.Lock()
			if t.closed {
				t.mu.Unlock()
				_ = reconnected.CloseDeadline(time.Now().Add(eventCloseTimeout))
				return nil, nil, nil, nil, fmt.Errorf("amqp event transport is closed")
			}
			if t.publisherConnection == nil {
				t.publisherConnection = reconnected
				t.sharedConnection = t.consumerConnection == reconnected
			}
			t.mu.Unlock()
			continue
		}

		channel, err := connection.Channel()
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("open event publisher channel: %w", err)
		}
		if err := channel.Confirm(false); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("enable event publisher confirms: %w", err)
		}
		returns := channel.NotifyReturn(make(chan amqp.Return, 1))
		closed := channel.NotifyClose(make(chan *amqp.Error, 1))

		t.mu.Lock()
		if t.closed || t.publisherConnection != connection {
			t.mu.Unlock()
			return nil, nil, nil, nil, fmt.Errorf("amqp event transport is closed")
		}
		t.publisher = channel
		t.publisherReturns = returns
		t.publisherClosed = closed
		t.mu.Unlock()
		return channel, returns, closed, closing, nil
	}
}

func (t *amqpEventTransport) topologyChannel() (eventChannel, error) {
	t.mu.Lock()
	if t.closed || t.consumerConnection == nil {
		t.mu.Unlock()
		return nil, fmt.Errorf("amqp event transport is closed")
	}
	if t.topology != nil && !t.topology.IsClosed() {
		channel := t.topology
		t.mu.Unlock()
		return channel, nil
	}
	t.topology = nil
	connection := t.consumerConnection
	t.mu.Unlock()

	channel, err := connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("open event topology channel: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.consumerConnection != connection {
		return nil, fmt.Errorf("amqp event transport is closed")
	}
	t.topology = channel
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

func (t *amqpEventTransport) invalidateTopology(channel eventChannel) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.topology == channel {
		t.topology = nil
	}
}

func (t *amqpEventTransport) invalidatePublisherLocked() {
	t.publisher = nil
	t.publisherReturns = nil
	t.publisherClosed = nil
}

func (t *amqpEventTransport) retirePublisherConnection(channel eventChannel) error {
	t.mu.Lock()
	if t.publisher != channel {
		t.mu.Unlock()
		return nil
	}
	connection := t.publisherConnection
	t.invalidatePublisherLocked()
	t.publisherConnection = nil
	t.mu.Unlock()

	if connection != nil {
		return connection.CloseDeadline(time.Now().Add(eventCloseTimeout))
	}
	return nil
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
	if t.closed || t.consumerConnection == nil {
		t.mu.Unlock()
		return fmt.Errorf("amqp event transport is closed")
	}
	connection := t.consumerConnection
	t.mu.Unlock()
	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("open event consumer channel: %w", err)
	}
	topology := topologyFor(subscription)
	consumerTag := "sparkops-event-" + normalizeQueueNamePart(subscription.Subscriber, false)
	deliveries, err := channel.Consume(topology.Main.Name, consumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume event queue %s: %w", topology.Main.Name, err)
	}
	t.mu.Lock()
	if t.closed || t.consumerConnection != connection {
		t.mu.Unlock()
		return fmt.Errorf("amqp event transport is closed")
	}
	t.consumers[channel] = consumerTag
	t.mu.Unlock()

	go t.consumeDeliveries(ctx, channel, consumerTag, subscription, deliveries, handler)
	return nil
}

func (t *amqpEventTransport) consumeDeliveries(
	ctx context.Context,
	channel eventChannel,
	consumerTag string,
	subscription Subscription,
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
				reason := fmt.Sprintf("cannot parse event envelope: %v", err)
				t.routeRawPoison(ctx, subscription, delivery, reason)
				continue
			}
			if err := validateEventEnvelope(envelope); err != nil {
				t.routeRawPoison(
					ctx,
					subscription,
					delivery,
					fmt.Sprintf("invalid event envelope: %v", err),
				)
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

func (t *amqpEventTransport) routeRawPoison(
	ctx context.Context,
	subscription Subscription,
	delivery amqp.Delivery,
	reason string,
) {
	if err := t.publishRawDeadLetter(ctx, subscription, delivery, reason); err != nil {
		_ = delivery.Nack(false, true)
		return
	}
	if t.metrics != nil {
		t.metrics.EventDLQ.WithLabelValues(subscription.Subscriber).Inc()
	}
	_ = delivery.Ack(false)
}

func validateEventEnvelope(envelope events.Envelope) error {
	if _, err := uuid.Parse(envelope.ID); err != nil {
		return fmt.Errorf("id must be a UUID")
	}
	if strings.TrimSpace(envelope.Type) == "" {
		return fmt.Errorf("type is required")
	}
	if envelope.Version <= 0 {
		return fmt.Errorf("version must be positive")
	}
	if envelope.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	payload := bytes.TrimSpace(envelope.Payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("null")) {
		return fmt.Errorf("payload is required")
	}
	return nil
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
	publisherConnection := t.publisherConnection
	consumerConnection := t.consumerConnection
	sharedConnection := t.sharedConnection
	close(t.closing)
	t.publisher = nil
	t.publisherConnection = nil
	t.consumerConnection = nil
	t.publisherReturns = nil
	t.publisherClosed = nil
	t.topology = nil
	t.consumers = make(map[eventChannel]string)
	t.mu.Unlock()

	deadline := time.Now().Add(eventCloseTimeout)
	var closeErrors []error
	if publisherConnection != nil {
		if err := publisherConnection.CloseDeadline(deadline); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close event publisher connection: %w", err))
		}
	}
	if consumerConnection != nil && !sharedConnection {
		if err := consumerConnection.CloseDeadline(deadline); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close event consumer connection: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}
