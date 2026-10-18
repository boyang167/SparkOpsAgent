package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	transport := newAMQPEventTransportWithFactory(connections[0], func(context.Context) (eventConnection, error) {
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
		func(context.Context) (eventConnection, error) {
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

	consumedEnvelope := eventTransportTestEnvelope()
	consumedBody, err := json.Marshal(consumedEnvelope)
	require.NoError(t, err)
	consumerChannel.deliveries <- amqp.Delivery{Body: consumedBody}
	select {
	case envelope := <-handled:
		require.Equal(t, consumedEnvelope.ID, envelope.ID)
	case <-time.After(time.Second):
		t.Fatal("consumer stopped after publisher timeout")
	}

	require.NoError(t, transport.Publish(context.Background(), eventTransportTestEnvelope()))
	require.Equal(t, 1, reconnects)
}

func TestEventTransportRoutesMalformedRawDeliveryToSubscriberDLQBeforeAck(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(confirmation.done)
	publisherChannel := newStubEventChannel(confirmation)
	consumerChannel := newStubEventChannel(nil)
	consumerChannel.deliveries = make(chan amqp.Delivery, 1)
	transport := newAMQPEventTransportWithConnections(
		&stubEventConnection{channels: []eventChannel{publisherChannel}},
		&stubEventConnection{channels: []eventChannel{consumerChannel}},
		nil,
	)
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	transport.metrics = metrics
	subscription := Subscription{
		EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "poison.consumer",
	}
	handlerCalled := make(chan struct{}, 1)
	require.NoError(t, transport.Consume(context.Background(), subscription, func(
		context.Context,
		events.Envelope,
		int,
	) error {
		handlerCalled <- struct{}{}
		return nil
	}))
	acks := newRecordingAcknowledger()
	body := []byte(`{"id":"poison","occurred_at":"not-a-time"}`)
	consumerChannel.deliveries <- amqp.Delivery{
		Acknowledger: acks,
		DeliveryTag:  7,
		Body:         body,
		ContentType:  "application/json",
		MessageId:    "raw-message-id",
		Headers:      amqp.Table{"source": "untrusted"},
	}

	select {
	case ack := <-acks.acked:
		require.Equal(t, acknowledgement{tag: 7}, ack)
	case nack := <-acks.nacked:
		t.Fatalf("malformed delivery was nacked before DLQ publish: %+v", nack)
	case <-time.After(time.Second):
		t.Fatal("malformed delivery was neither acked nor nacked")
	}
	require.Empty(t, acks.nacked)
	require.Empty(t, handlerCalled)
	require.Len(t, publisherChannel.publishedRecords, 1)
	published := publisherChannel.publishedRecords[0]
	require.Empty(t, published.exchange)
	require.Equal(t, topologyFor(subscription).DeadLetter.Name, published.key)
	require.Equal(t, body, published.message.Body)
	require.Equal(t, "raw-message-id", published.message.MessageId)
	require.Equal(t, "untrusted", published.message.Headers["source"])
	require.Contains(t, published.message.Headers["x-sparkops-dead-letter-reason"], "cannot parse")
	require.Equal(t, float64(1), prometheusMetricValue(
		t,
		metrics.EventDLQ.WithLabelValues(subscription.Subscriber),
	))
}

func TestEventTransportPublishesStructuredDLQWithCurrentAttempts(t *testing.T) {
	confirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(confirmation.done)
	channel := newStubEventChannel(confirmation)
	transport := newAMQPEventTransport(&stubEventConnection{channels: []eventChannel{channel}})
	subscription := Subscription{
		EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "retry.consumer",
	}

	require.NoError(t, transport.PublishDeadLetter(
		context.Background(),
		subscription,
		eventTransportTestEnvelope(),
		"attempts exhausted",
		4,
	))

	require.Len(t, channel.publishedRecords, 1)
	published := channel.publishedRecords[0]
	require.Equal(t, topologyFor(subscription).DeadLetter.Name, published.key)
	require.EqualValues(t, 4, published.message.Headers[eventAttemptsHeader])
	require.Equal(t, "attempts exhausted", published.message.Headers["x-sparkops-dead-letter-reason"])
}

func TestEventTransportRoutesStructurallyInvalidEnvelopeToSubscriberDLQ(t *testing.T) {
	valid := eventTransportTestEnvelope()
	valid.Payload = json.RawMessage(`{"build_id":"build-123"}`)
	tests := []struct {
		name   string
		mutate func(*events.Envelope)
	}{
		{name: "empty id", mutate: func(envelope *events.Envelope) { envelope.ID = "" }},
		{name: "non uuid id", mutate: func(envelope *events.Envelope) { envelope.ID = "not-a-uuid" }},
		{name: "empty type", mutate: func(envelope *events.Envelope) { envelope.Type = "" }},
		{name: "blank type", mutate: func(envelope *events.Envelope) { envelope.Type = "  " }},
		{name: "type too long", mutate: func(envelope *events.Envelope) {
			envelope.Type = strings.Repeat("a", 161)
		}},
		{name: "type contains nul", mutate: func(envelope *events.Envelope) {
			envelope.Type = "delivery.\x00BuildCompleted"
		}},
		{name: "type mismatches subscription", mutate: func(envelope *events.Envelope) {
			envelope.Type = "delivery.DeployCompleted"
		}},
		{name: "zero version", mutate: func(envelope *events.Envelope) { envelope.Version = 0 }},
		{name: "negative version", mutate: func(envelope *events.Envelope) { envelope.Version = -1 }},
		{name: "version mismatches subscription", mutate: func(envelope *events.Envelope) {
			envelope.Version = 2
		}},
		{name: "missing occurred at", mutate: func(envelope *events.Envelope) { envelope.OccurredAt = time.Time{} }},
		{name: "missing payload", mutate: func(envelope *events.Envelope) { envelope.Payload = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := valid
			tt.mutate(&envelope)
			body, err := json.Marshal(envelope)
			require.NoError(t, err)
			confirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
			close(confirmation.done)
			publisherChannel := newStubEventChannel(confirmation)
			consumerChannel := newStubEventChannel(nil)
			consumerChannel.deliveries = make(chan amqp.Delivery, 1)
			transport := newAMQPEventTransportWithConnections(
				&stubEventConnection{channels: []eventChannel{publisherChannel}},
				&stubEventConnection{channels: []eventChannel{consumerChannel}},
				nil,
			)
			subscription := Subscription{
				EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "poison.consumer",
			}
			handlerCalled := make(chan struct{}, 1)
			require.NoError(t, transport.Consume(context.Background(), subscription, func(
				context.Context,
				events.Envelope,
				int,
			) error {
				handlerCalled <- struct{}{}
				return nil
			}))
			acks := newRecordingAcknowledger()
			consumerChannel.deliveries <- amqp.Delivery{
				Acknowledger: acks,
				DeliveryTag:  8,
				Body:         body,
				MessageId:    "raw-message-id",
			}

			require.Equal(t, acknowledgement{tag: 8}, <-acks.acked)
			require.Empty(t, acks.nacked)
			require.Empty(t, handlerCalled)
			require.Len(t, publisherChannel.publishedRecords, 1)
			published := publisherChannel.publishedRecords[0]
			require.Equal(t, body, published.message.Body)
			require.Contains(t, published.message.Headers["x-sparkops-dead-letter-reason"], "invalid event envelope")
		})
	}
}

func TestEventTransportNacksMalformedRawDeliveryWhenDLQConfirmFails(t *testing.T) {
	tests := []struct {
		name         string
		confirmation eventConfirmation
		publishErr   error
	}{
		{
			name: "publisher confirm nack",
			confirmation: func() eventConfirmation {
				value := &stubEventConfirmation{done: make(chan struct{}), acked: false}
				close(value.done)
				return value
			}(),
		},
		{name: "publish failure", publishErr: errors.New("publisher unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisherChannel := newStubEventChannel(tt.confirmation)
			publisherChannel.publishError = tt.publishErr
			consumerChannel := newStubEventChannel(nil)
			consumerChannel.deliveries = make(chan amqp.Delivery, 1)
			transport := newAMQPEventTransportWithConnections(
				&stubEventConnection{channels: []eventChannel{publisherChannel}},
				&stubEventConnection{channels: []eventChannel{consumerChannel}},
				nil,
			)
			subscription := Subscription{
				EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "poison.consumer",
			}
			require.NoError(t, transport.Consume(context.Background(), subscription, func(
				context.Context,
				events.Envelope,
				int,
			) error {
				t.Fatal("malformed raw delivery reached decoded handler")
				return nil
			}))
			acks := newRecordingAcknowledger()
			consumerChannel.deliveries <- amqp.Delivery{
				Acknowledger: acks,
				DeliveryTag:  9,
				Body:         []byte(`{"id":`),
			}

			require.Equal(t, acknowledgement{tag: 9, requeue: true}, <-acks.nacked)
			require.Empty(t, acks.acked)
			require.Equal(t, 1, publisherChannel.publishCalls)
		})
	}
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

func TestEventTransportRedialsPublisherWhenConnectionCannotOpenChannel(t *testing.T) {
	initialPublisher := &stubEventConnection{}
	consumerConnection := &stubEventConnection{}
	confirmation := &stubEventConfirmation{done: make(chan struct{}), acked: true}
	close(confirmation.done)
	recoveredPublisher := &stubEventConnection{
		channels: []eventChannel{newStubEventChannel(confirmation)},
	}
	reconnects := 0
	transport := newAMQPEventTransportWithConnections(
		initialPublisher,
		consumerConnection,
		func(context.Context) (eventConnection, error) {
			reconnects++
			return recoveredPublisher, nil
		},
	)
	t.Cleanup(func() { _ = transport.Close() })

	require.NoError(t, transport.Publish(context.Background(), eventTransportTestEnvelope()))
	require.Equal(t, 1, reconnects)
	require.Equal(t, 1, initialPublisher.closeDeadlineCalls)
	require.Equal(t, 1, recoveredPublisher.channelCalls)
}

func TestEventTransportReconnectsConsumerConnectionAndRestoresEverySubscription(t *testing.T) {
	publisherConnection := &stubEventConnection{}
	initialTopology := newStubEventChannel(nil)
	initialOne := newStubEventChannel(nil)
	initialOne.deliveries = make(chan amqp.Delivery, 1)
	initialTwo := newStubEventChannel(nil)
	initialTwo.deliveries = make(chan amqp.Delivery, 1)
	initialConsumer := &stubEventConnection{channels: []eventChannel{
		initialTopology, initialOne, initialTwo,
	}}
	recoveredTopology := newStubEventChannel(nil)
	recoveredOne := newStubEventChannel(nil)
	recoveredOne.deliveries = make(chan amqp.Delivery, 1)
	recoveredTwo := newStubEventChannel(nil)
	recoveredTwo.deliveries = make(chan amqp.Delivery, 1)
	recoveredConsumer := &stubEventConnection{channels: []eventChannel{
		recoveredTopology, recoveredOne, recoveredTwo,
	}}
	reconnects := make(chan struct{}, 1)
	transport := newAMQPEventTransportWithConnections(
		publisherConnection,
		initialConsumer,
		func(context.Context) (eventConnection, error) {
			reconnects <- struct{}{}
			return recoveredConsumer, nil
		},
	)
	t.Cleanup(func() { _ = transport.Close() })

	one := Subscription{EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "consumer.one"}
	two := Subscription{EventName: "delivery.BuildCompleted", Version: 1, Subscriber: "consumer.two"}
	require.NoError(t, transport.Declare(context.Background(), []Subscription{one, two}))
	handledOne := make(chan string, 1)
	handledTwo := make(chan string, 1)
	require.NoError(t, transport.Consume(context.Background(), one, func(_ context.Context, envelope events.Envelope, _ int) error {
		handledOne <- envelope.ID
		return nil
	}))
	require.NoError(t, transport.Consume(context.Background(), two, func(_ context.Context, envelope events.Envelope, _ int) error {
		handledTwo <- envelope.ID
		return nil
	}))

	close(initialOne.deliveries)
	close(initialTwo.deliveries)
	select {
	case <-reconnects:
	case <-time.After(time.Second):
		t.Fatal("consumer connection was not redialed")
	}
	require.Eventually(t, func() bool {
		return recoveredConsumer.channelCalls == 3
	}, time.Second, time.Millisecond)

	envelope := eventTransportTestEnvelope()
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	recoveredOne.deliveries <- amqp.Delivery{Body: body}
	recoveredTwo.deliveries <- amqp.Delivery{Body: body}
	require.Equal(t, envelope.ID, <-handledOne)
	require.Equal(t, envelope.ID, <-handledTwo)
	require.Equal(t, 1, initialConsumer.closeDeadlineCalls)
	require.Equal(t, 1, recoveredTopology.exchangeDeclareCalls)
	require.Equal(t, 2, recoveredTopology.queueBindCalls)
}

func TestEventTransportCloseInterruptsConsumerReconnectBackoff(t *testing.T) {
	initialTopology := newStubEventChannel(nil)
	initialConsumerChannel := newStubEventChannel(nil)
	initialConsumerChannel.deliveries = make(chan amqp.Delivery)
	initialConsumer := &stubEventConnection{channels: []eventChannel{initialTopology, initialConsumerChannel}}
	attempted := make(chan struct{}, 1)
	transport := newAMQPEventTransportWithConnections(
		&stubEventConnection{},
		initialConsumer,
		func(ctx context.Context) (eventConnection, error) {
			select {
			case attempted <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)
	subscription := Subscription{
		EventName:  "delivery.BuildCompleted",
		Version:    1,
		Subscriber: "consumer.close-during-reconnect",
	}
	require.NoError(t, transport.Declare(context.Background(), []Subscription{subscription}))
	require.NoError(t, transport.Consume(context.Background(), subscription, func(context.Context, events.Envelope, int) error {
		return nil
	}))

	close(initialConsumerChannel.deliveries)
	select {
	case <-attempted:
	case <-time.After(time.Second):
		t.Fatal("consumer reconnect was not attempted")
	}

	closed := make(chan error, 1)
	go func() { closed <- transport.Close() }()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close did not interrupt reconnect backoff")
	}
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
	publishedRecords          []recordedEventPublish
	deliveries                chan amqp.Delivery
	exchangeDeclareCalls      int
	queueBindCalls            int
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
	exchange string,
	key string,
	mandatory bool,
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
	c.publishedRecords = append(c.publishedRecords, recordedEventPublish{
		exchange: exchange, key: key, mandatory: mandatory, message: message,
	})
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

type acknowledgement struct {
	tag     uint64
	requeue bool
}

type recordingAcknowledger struct {
	acked  chan acknowledgement
	nacked chan acknowledgement
}

func newRecordingAcknowledger() *recordingAcknowledger {
	return &recordingAcknowledger{
		acked:  make(chan acknowledgement, 1),
		nacked: make(chan acknowledgement, 1),
	}
}

func (a *recordingAcknowledger) Ack(tag uint64, _ bool) error {
	a.acked <- acknowledgement{tag: tag}
	return nil
}

func (a *recordingAcknowledger) Nack(tag uint64, _ bool, requeue bool) error {
	a.nacked <- acknowledgement{tag: tag, requeue: requeue}
	return nil
}

func (*recordingAcknowledger) Reject(uint64, bool) error { return nil }

func (*stubEventChannel) Confirm(bool) error { return nil }

func (c *stubEventChannel) ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error {
	c.exchangeDeclareCalls++
	return nil
}

func (*stubEventChannel) QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error) {
	return amqp.Queue{}, nil
}

func (c *stubEventChannel) QueueBind(string, string, string, bool, amqp.Table) error {
	c.queueBindCalls++
	return nil
}

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
		ID:         "550e8400-e29b-41d4-a716-446655440000",
		Type:       "delivery.BuildCompleted",
		Version:    1,
		OccurredAt: time.Date(2026, 10, 17, 6, 0, 0, 0, time.UTC),
		Payload:    json.RawMessage(`{"build_id":"build-123"}`),
	}
}
