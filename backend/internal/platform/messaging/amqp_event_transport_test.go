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
	returns <- amqp.Return{ReplyCode: 312, ReplyText: "NO_ROUTE"}

	err := publishEventConfirmed(
		context.Background(),
		&stubEventPublishChannel{confirmation: &stubEventConfirmation{done: make(chan struct{})}},
		returns,
		make(chan *amqp.Error, 1),
		eventExchangeName,
		"delivery.BuildCompleted.v1",
		eventTransportTestEnvelope(),
		nil,
	)

	require.ErrorContains(t, err, "returned")
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
	close(firstConfirmation.done)

	require.ErrorContains(t, <-firstResult, "nack")
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
	publishStarted            chan struct{}
	returns                   chan amqp.Return
	closed                    chan *amqp.Error
	closeBlock                chan struct{}
	suppressCloseNotification bool
	closeCalls                int
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
	context.Context,
	string,
	string,
	bool,
	bool,
	amqp.Publishing,
) (eventConfirmation, error) {
	close(c.publishStarted)
	return c.confirmation, nil
}

func (*stubEventChannel) Confirm(bool) error { return nil }

func (*stubEventChannel) ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error {
	return nil
}

func (*stubEventChannel) QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error) {
	return amqp.Queue{}, nil
}

func (*stubEventChannel) QueueBind(string, string, string, bool, amqp.Table) error { return nil }

func (*stubEventChannel) Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error) {
	return make(chan amqp.Delivery), nil
}

func (*stubEventChannel) Cancel(string, bool) error { return nil }

func (c *stubEventChannel) NotifyReturn(chan amqp.Return) chan amqp.Return { return c.returns }

func (c *stubEventChannel) NotifyClose(chan *amqp.Error) chan *amqp.Error { return c.closed }

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
