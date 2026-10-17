package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
)

type stubEventConfirmation struct {
	done  chan struct{}
	acked bool
}

func (c *stubEventConfirmation) Done() <-chan struct{} { return c.done }
func (c *stubEventConfirmation) Acked() bool           { return c.acked }

type recordedEventPublish struct {
	exchange  string
	key       string
	mandatory bool
	message   amqp.Publishing
}

type stubEventPublishChannel struct {
	confirmation eventConfirmation
	published    recordedEventPublish
	err          error
}

func (c *stubEventPublishChannel) PublishWithDeferredConfirmWithContext(
	_ context.Context,
	exchange string,
	key string,
	mandatory bool,
	_ bool,
	message amqp.Publishing,
) (eventConfirmation, error) {
	c.published = recordedEventPublish{
		exchange: exchange, key: key, mandatory: mandatory, message: message,
	}
	return c.confirmation, c.err
}

func TestEventTransportPublishesPersistentMandatoryMessageAndWaitsForAck(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(confirmation.done)
	channel := &stubEventPublishChannel{confirmation: confirmation}
	envelope := eventTransportTestEnvelope()
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	err = publishEventConfirmed(
		context.Background(),
		channel,
		make(chan amqp.Return, 1),
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		envelope,
		nil,
	)

	require.NoError(t, err)
	require.Equal(t, eventExchangeName, channel.published.exchange)
	require.Equal(t, "delivery.BuildCompleted.v1", channel.published.key)
	require.True(t, channel.published.mandatory)
	require.Equal(t, amqp.Persistent, channel.published.message.DeliveryMode)
	require.Equal(t, "application/json", channel.published.message.ContentType)
	require.Equal(t, envelope.ID, channel.published.message.MessageId)
	require.Equal(t, envelope.OccurredAt, channel.published.message.Timestamp)
	require.JSONEq(t, string(body), string(channel.published.message.Body))
}

func TestEventTransportTreatsNackAsFailure(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{})}
	close(confirmation.done)

	err := publishEventConfirmed(
		context.Background(),
		&stubEventPublishChannel{confirmation: confirmation},
		make(chan amqp.Return, 1),
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		eventTransportTestEnvelope(),
		nil,
	)

	require.ErrorContains(t, err, "nack")
}

func TestEventTransportTreatsMandatoryReturnAsFailure(t *testing.T) {
	returns := make(chan amqp.Return, 1)
	envelope := eventTransportTestEnvelope()
	returns <- amqp.Return{ReplyCode: 312, ReplyText: "NO_ROUTE", MessageId: envelope.ID}

	err := publishEventConfirmed(
		context.Background(),
		&stubEventPublishChannel{confirmation: &stubEventConfirmation{done: make(chan struct{})}},
		returns,
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		envelope,
		nil,
	)

	require.ErrorContains(t, err, "returned")
}

func TestEventTransportIgnoresLateReturnForDifferentMessageID(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(confirmation.done)
	returns := make(chan amqp.Return, 1)
	returns <- amqp.Return{ReplyCode: 312, ReplyText: "NO_ROUTE", MessageId: "previous-event"}

	err := publishEventConfirmed(
		context.Background(),
		&stubEventPublishChannel{confirmation: confirmation},
		returns,
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		eventTransportTestEnvelope(),
		nil,
	)

	require.NoError(t, err)
}

func TestEventTransportIgnoresLateReturnFromEarlierAttemptWithSameMessageID(t *testing.T) {
	envelope := eventTransportTestEnvelope()
	returns := make(chan amqp.Return, 1)
	previousConfirmation := &stubEventConfirmation{done: make(chan struct{})}
	previousContext, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	previousHeaders := amqp.Table{"x-sparkops-publish-attempt": "attempt-1"}

	err := publishEventConfirmed(
		previousContext,
		&stubEventPublishChannel{confirmation: previousConfirmation},
		returns,
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		envelope,
		previousHeaders,
	)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	returns <- amqp.Return{
		ReplyCode: 312,
		ReplyText: "NO_ROUTE",
		MessageId: envelope.ID,
		Headers:   previousHeaders,
	}
	currentConfirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(currentConfirmation.done)
	err = publishEventConfirmed(
		context.Background(),
		&stubEventPublishChannel{confirmation: currentConfirmation},
		returns,
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		envelope,
		amqp.Table{"x-sparkops-publish-attempt": "attempt-2"},
	)

	require.NoError(t, err)
}

func TestEventTransportTreatsChannelCloseAsFailure(t *testing.T) {
	closed := make(chan *amqp.Error, 1)
	closed <- &amqp.Error{Code: 504, Reason: "channel closed"}

	err := publishEventConfirmed(
		context.Background(),
		&stubEventPublishChannel{confirmation: &stubEventConfirmation{done: make(chan struct{})}},
		make(chan amqp.Return, 1),
		closed,
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		eventTransportTestEnvelope(),
		nil,
	)

	require.ErrorContains(t, err, "closed")
}

func TestEventTransportTreatsContextTimeoutAsFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err := publishEventConfirmed(
		ctx,
		&stubEventPublishChannel{confirmation: &stubEventConfirmation{done: make(chan struct{})}},
		make(chan amqp.Return, 1),
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		eventTransportTestEnvelope(),
		nil,
	)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestEventTransportRejectsMissingDeferredConfirmation(t *testing.T) {
	err := publishEventConfirmed(
		context.Background(),
		&stubEventPublishChannel{},
		make(chan amqp.Return, 1),
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		eventTransportTestEnvelope(),
		nil,
	)

	require.ErrorContains(t, err, "confirm")
}

func TestEventTransportReturnsPublishError(t *testing.T) {
	publishErr := errors.New("socket write failed")
	err := publishEventConfirmed(
		context.Background(),
		&stubEventPublishChannel{err: publishErr},
		make(chan amqp.Return, 1),
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		eventTransportTestEnvelope(),
		nil,
	)

	require.ErrorIs(t, err, publishErr)
}

func TestEventTransportCloseUnblocksPendingPublish(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{})}
	channel := newStubEventChannel(confirmation)
	transport := newAMQPEventTransport(&stubEventConnection{channels: []eventChannel{channel}})
	t.Cleanup(func() {
		select {
		case <-confirmation.done:
		default:
			close(confirmation.done)
		}
		_ = transport.Close()
	})

	publishResult := make(chan error, 1)
	go func() {
		publishResult <- transport.Publish(context.Background(), eventTransportTestEnvelope())
	}()
	<-channel.publishStarted

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- transport.Close()
	}()

	select {
	case err := <-closeResult:
		require.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close blocked while publish waited for a confirmation")
	}

	select {
	case err := <-publishResult:
		require.ErrorContains(t, err, "closed")
	case <-time.After(time.Second):
		t.Fatal("pending publish was not unblocked by Close")
	}
}

func TestEventTransportCloseDoesNotWaitForChannelClose(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{})}
	channel := newStubEventChannel(confirmation)
	channel.closeBlock = make(chan struct{})
	channel.suppressCloseNotification = true
	connection := &stubEventConnection{channels: []eventChannel{channel}}
	transport := newAMQPEventTransport(connection)
	t.Cleanup(func() {
		close(channel.closeBlock)
		close(confirmation.done)
	})

	publishResult := make(chan error, 1)
	go func() {
		publishResult <- transport.Publish(context.Background(), eventTransportTestEnvelope())
	}()
	<-channel.publishStarted

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- transport.Close()
	}()

	select {
	case err := <-closeResult:
		require.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close waited for synchronous channel close")
	}

	select {
	case err := <-publishResult:
		require.ErrorContains(t, err, "transport closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("pending publish did not observe transport closing")
	}
	require.Equal(t, 1, connection.closeDeadlineCalls)
}

func TestEventTransportFailedPublishDoesNotWaitForChannelClose(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{})}
	close(confirmation.done)
	channel := newStubEventChannel(confirmation)
	channel.closeBlock = make(chan struct{})
	transport := newAMQPEventTransport(&stubEventConnection{channels: []eventChannel{channel}})
	t.Cleanup(func() { close(channel.closeBlock) })

	result := make(chan error, 1)
	go func() {
		result <- transport.Publish(context.Background(), eventTransportTestEnvelope())
	}()

	select {
	case err := <-result:
		require.ErrorContains(t, err, "nack")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("failed publish waited for synchronous channel close")
	}
	require.Zero(t, channel.closeCalls)
}

func TestEventTransportReusesHealthyPublisherAfterMessageFailures(t *testing.T) {
	confirmations := make([]eventConfirmation, 3)
	for index := range confirmations {
		confirmation := &stubEventConfirmation{done: make(chan struct{})}
		close(confirmation.done)
		confirmations[index] = confirmation
	}
	channel := newStubEventChannel(confirmations[0])
	channel.confirmations = confirmations
	connection := &stubEventConnection{channels: []eventChannel{channel}}
	transport := newAMQPEventTransport(connection)

	for range confirmations {
		require.ErrorContains(t, transport.Publish(context.Background(), eventTransportTestEnvelope()), "nack")
	}

	require.Equal(t, 1, connection.channelCalls)
	require.Equal(t, 1, channel.returnListenerCalls)
	require.Equal(t, 1, channel.closeListenerCalls)
}

func TestEventTransportReusesHealthyPublisherAfterMandatoryReturns(t *testing.T) {
	pendingConfirmation := &stubEventConfirmation{done: make(chan struct{})}
	channel := newStubEventChannel(pendingConfirmation)
	connection := &stubEventConnection{channels: []eventChannel{channel}}
	transport := newAMQPEventTransport(connection)
	envelope := eventTransportTestEnvelope()
	channel.onPublish = func(message amqp.Publishing) {
		channel.returns <- amqp.Return{
			ReplyCode: 312,
			ReplyText: "NO_ROUTE",
			MessageId: message.MessageId,
			Headers:   message.Headers,
		}
	}

	for range 3 {
		require.ErrorContains(t, transport.Publish(context.Background(), envelope), "returned")
	}

	require.Equal(t, 1, connection.channelCalls)
	require.Equal(t, 1, channel.returnListenerCalls)
	require.Equal(t, 1, channel.closeListenerCalls)
}

func TestEventTransportRetiresConfirmationSessionsAfterTimeout(t *testing.T) {
	pendingConfirmations := make([]*stubEventConfirmation, 3)
	connections := make([]*stubEventConnection, 0, len(pendingConfirmations)+1)
	for index := range pendingConfirmations {
		pendingConfirmations[index] = &stubEventConfirmation{done: make(chan struct{})}
		connections = append(connections, &stubEventConnection{
			channels: []eventChannel{newStubEventChannel(pendingConfirmations[index])},
		})
	}
	finalConfirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(finalConfirmation.done)
	connections = append(connections, &stubEventConnection{
		channels: []eventChannel{newStubEventChannel(finalConfirmation)},
	})

	nextConnection := 1
	transport := newAMQPEventTransportWithFactory(connections[0], func() (eventConnection, error) {
		connection := connections[nextConnection]
		nextConnection++
		return connection, nil
	})

	for range pendingConfirmations {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		require.ErrorIs(t, transport.Publish(ctx, eventTransportTestEnvelope()), context.DeadlineExceeded)
		cancel()
	}
	require.NoError(t, transport.Publish(context.Background(), eventTransportTestEnvelope()))

	for _, connection := range connections[:len(connections)-1] {
		require.Equal(t, 1, connection.channelCalls)
		require.Equal(t, 1, connection.closeDeadlineCalls)
	}
	require.Equal(t, 1, connections[len(connections)-1].channelCalls)
	require.Equal(t, len(pendingConfirmations), nextConnection-1)
}

func TestEventTransportPublisherTimeoutKeepsConsumerConnectionAlive(t *testing.T) {
	pendingConfirmation := &stubEventConfirmation{done: make(chan struct{})}
	publisherConnection := &stubEventConnection{
		channels: []eventChannel{newStubEventChannel(pendingConfirmation)},
	}
	consumerChannel := newStubEventChannel(nil)
	consumerChannel.deliveries = make(chan amqp.Delivery, 1)
	consumerConnection := &stubEventConnection{
		channels: []eventChannel{consumerChannel},
	}
	reconnectedConfirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(reconnectedConfirmation.done)
	reconnectedPublisher := &stubEventConnection{
		channels: []eventChannel{newStubEventChannel(reconnectedConfirmation)},
	}
	reconnects := 0
	transport := newAMQPEventTransportWithConnections(
		publisherConnection,
		consumerConnection,
		func() (eventConnection, error) {
			reconnects++
			return reconnectedPublisher, nil
		},
	)

	handled := make(chan events.Envelope, 1)
	consumeContext, cancelConsume := context.WithCancel(context.Background())
	defer cancelConsume()
	require.NoError(t, transport.Consume(
		consumeContext,
		Subscription{EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "consumer"},
		func(_ context.Context, envelope events.Envelope, _ int) error {
			handled <- envelope
			return nil
		},
	))

	publishContext, cancelPublish := context.WithTimeout(context.Background(), time.Millisecond)
	require.ErrorIs(t, transport.Publish(publishContext, eventTransportTestEnvelope()), context.DeadlineExceeded)
	cancelPublish()
	require.Equal(t, 1, publisherConnection.closeDeadlineCalls)
	require.Zero(t, consumerConnection.closeDeadlineCalls)

	consumerChannel.deliveries <- amqp.Delivery{Body: []byte(`{"id":"event-123","type":"delivery.BuildCompleted","version":1}`)}
	select {
	case envelope := <-handled:
		require.Equal(t, "event-123", envelope.ID)
	case <-time.After(time.Second):
		t.Fatal("consumer stopped after publisher timeout")
	}

	require.NoError(t, transport.Publish(context.Background(), eventTransportTestEnvelope()))
	require.Equal(t, 1, reconnects)
}

func TestEventTransportDeclareUsesConsumerConnection(t *testing.T) {
	publisherConnection := &stubEventConnection{
		channels: []eventChannel{newStubEventChannel(nil)},
	}
	topologyChannel := newStubEventChannel(nil)
	consumerConnection := &stubEventConnection{
		channels: []eventChannel{topologyChannel},
	}
	transport := newAMQPEventTransportWithConnections(publisherConnection, consumerConnection, nil)

	require.NoError(t, transport.Declare(context.Background(), []Subscription{{
		EventName:  "delivery.BuildCompleted",
		Version:    1,
		Subscriber: "consumer",
	}}))
	require.Zero(t, publisherConnection.channelCalls)
	require.Equal(t, 1, consumerConnection.channelCalls)
}

func TestEventTransportCloseClosesPublisherAndConsumerConnections(t *testing.T) {
	publisherConnection := &stubEventConnection{}
	consumerConnection := &stubEventConnection{}
	transport := newAMQPEventTransportWithConnections(publisherConnection, consumerConnection, nil)

	require.NoError(t, transport.Close())
	require.NoError(t, transport.Close())
	require.Equal(t, 1, publisherConnection.closeDeadlineCalls)
	require.Equal(t, 1, consumerConnection.closeDeadlineCalls)
}

func TestEventTransportKeepsEventIDAndGeneratesUniqueAttemptHeaders(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{})}
	channel := newStubEventChannel(confirmation)
	connection := &stubEventConnection{channels: []eventChannel{channel}}
	transport := newAMQPEventTransport(connection)
	envelope := eventTransportTestEnvelope()
	channel.onPublish = func(message amqp.Publishing) {
		channel.returns <- amqp.Return{
			ReplyCode: 312,
			ReplyText: "NO_ROUTE",
			MessageId: message.MessageId,
			Headers:   message.Headers,
		}
	}

	require.ErrorContains(t, transport.Publish(context.Background(), envelope), "returned")
	require.ErrorContains(t, transport.Publish(context.Background(), envelope), "returned")
	require.Len(t, channel.published, 2)
	require.Equal(t, envelope.ID, channel.published[0].MessageId)
	require.Equal(t, envelope.ID, channel.published[1].MessageId)
	require.NotEqual(
		t,
		channel.published[0].Headers[eventPublishAttemptHeader],
		channel.published[1].Headers[eventPublishAttemptHeader],
	)
}

func TestEventTransportReusesHealthyPublisherAfterPublishError(t *testing.T) {
	channel := newStubEventChannel(nil)
	channel.publishError = errors.New("temporary write error")
	connection := &stubEventConnection{channels: []eventChannel{channel}}
	transport := newAMQPEventTransport(connection)

	for range 3 {
		require.ErrorIs(t, transport.Publish(context.Background(), eventTransportTestEnvelope()), channel.publishError)
	}

	require.Equal(t, 1, connection.channelCalls)
}

func TestEventTransportRecreatesPublisherAfterNotifyClose(t *testing.T) {
	firstConfirmation := &stubEventConfirmation{done: make(chan struct{})}
	firstChannel := newStubEventChannel(firstConfirmation)
	firstChannel.closed <- &amqp.Error{Code: 504, Reason: "channel closed"}
	secondConfirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(secondConfirmation.done)
	secondChannel := newStubEventChannel(secondConfirmation)
	connection := &stubEventConnection{channels: []eventChannel{firstChannel, secondChannel}}
	transport := newAMQPEventTransport(connection)

	require.ErrorContains(t, transport.Publish(context.Background(), eventTransportTestEnvelope()), "closed")
	require.NoError(t, transport.Publish(context.Background(), eventTransportTestEnvelope()))
	require.Equal(t, 2, connection.channelCalls)
}

func TestEventTransportCloseAfterConfirmedPublishIsIdempotent(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(confirmation.done)
	channel := newStubEventChannel(confirmation)
	transport := newAMQPEventTransport(&stubEventConnection{channels: []eventChannel{channel}})

	require.NoError(t, transport.Publish(context.Background(), eventTransportTestEnvelope()))
	require.NoError(t, transport.Close())
	require.NoError(t, transport.Close())
	require.ErrorContains(t, transport.Publish(context.Background(), eventTransportTestEnvelope()), "closed")
}

func TestEventTransportRecreatesPublisherAfterFailedConcurrentPublish(t *testing.T) {
	firstConfirmation := &stubEventConfirmation{done: make(chan struct{})}
	firstChannel := newStubEventChannel(firstConfirmation)
	secondConfirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(secondConfirmation.done)
	secondChannel := newStubEventChannel(secondConfirmation)
	connection := &stubEventConnection{channels: []eventChannel{firstChannel, secondChannel}}
	transport := newAMQPEventTransport(connection)
	t.Cleanup(func() { _ = transport.Close() })

	firstResult := make(chan error, 1)
	go func() {
		firstResult <- transport.Publish(context.Background(), eventTransportTestEnvelope())
	}()
	<-firstChannel.publishStarted

	secondResult := make(chan error, 1)
	go func() {
		secondResult <- transport.Publish(context.Background(), eventTransportTestEnvelope())
	}()
	firstChannel.closed <- &amqp.Error{Code: 504, Reason: "channel closed"}

	require.ErrorContains(t, <-firstResult, "closed")
	require.NoError(t, <-secondResult)
	require.Equal(t, 2, connection.channelCalls)
}

type stubEventConnection struct {
	channels           []eventChannel
	channelCalls       int
	closeDeadlineCalls int
}

func (c *stubEventConnection) Channel() (eventChannel, error) {
	if len(c.channels) == 0 {
		return nil, errors.New("no event channels")
	}
	channel := c.channels[0]
	c.channels = c.channels[1:]
	c.channelCalls++
	return channel, nil
}

func (*stubEventConnection) Close() error { return nil }

func (c *stubEventConnection) CloseDeadline(time.Time) error {
	c.closeDeadlineCalls++
	return nil
}

type stubEventChannel struct {
	confirmation              eventConfirmation
	confirmations             []eventConfirmation
	publishCalls              int
	publishStarted            chan struct{}
	returns                   chan amqp.Return
	closed                    chan *amqp.Error
	closeBlock                chan struct{}
	suppressCloseNotification bool
	closeCalls                int
	returnListenerCalls       int
	closeListenerCalls        int
	isClosed                  bool
	publishError              error
	onPublish                 func(amqp.Publishing)
	published                 []amqp.Publishing
	deliveries                chan amqp.Delivery
}

func newStubEventChannel(confirmation eventConfirmation) *stubEventChannel {
	return &stubEventChannel{
		confirmation:   confirmation,
		publishStarted: make(chan struct{}),
		returns:        make(chan amqp.Return, 1),
		closed:         make(chan *amqp.Error, 1),
	}
}

func (c *stubEventChannel) PublishWithDeferredConfirmWithContext(
	_ context.Context,
	_ string,
	_ string,
	_ bool,
	_ bool,
	message amqp.Publishing,
) (eventConfirmation, error) {
	if c.publishCalls == 0 {
		close(c.publishStarted)
	}
	if c.publishError != nil {
		c.publishCalls++
		return nil, c.publishError
	}
	c.published = append(c.published, message)
	if c.onPublish != nil {
		c.onPublish(message)
	}
	confirmation := c.confirmation
	if c.publishCalls < len(c.confirmations) {
		confirmation = c.confirmations[c.publishCalls]
	}
	c.publishCalls++
	return confirmation, nil
}

func (*stubEventChannel) Confirm(bool) error { return nil }

func (*stubEventChannel) ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error {
	return nil
}

func (*stubEventChannel) QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error) {
	return amqp.Queue{}, nil
}

func (*stubEventChannel) QueueBind(string, string, string, bool, amqp.Table) error { return nil }

func (c *stubEventChannel) Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error) {
	if c.deliveries != nil {
		return c.deliveries, nil
	}
	return make(chan amqp.Delivery), nil
}

func (*stubEventChannel) Cancel(string, bool) error { return nil }

func (c *stubEventChannel) NotifyReturn(chan amqp.Return) chan amqp.Return {
	c.returnListenerCalls++
	return c.returns
}

func (c *stubEventChannel) NotifyClose(chan *amqp.Error) chan *amqp.Error {
	c.closeListenerCalls++
	return c.closed
}

func (c *stubEventChannel) IsClosed() bool { return c.isClosed }

func (c *stubEventChannel) Close() error {
	c.closeCalls++
	if c.closeBlock != nil {
		<-c.closeBlock
	}
	if c.suppressCloseNotification {
		return nil
	}
	select {
	case c.closed <- &amqp.Error{Reason: "closed by transport"}:
	default:
	}
	return nil
}

func eventTransportTestEnvelope() events.Envelope {
	return events.Envelope{
		ID:         "event-123",
		Type:       "delivery.BuildCompleted",
		Version:    1,
		OccurredAt: time.Date(2026, 10, 17, 6, 0, 0, 0, time.UTC),
		Payload:    json.RawMessage(`{"build_id":"build-123"}`),
	}
}
