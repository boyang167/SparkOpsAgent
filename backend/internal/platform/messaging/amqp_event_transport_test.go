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

func eventTransportTestEnvelope() events.Envelope {
	return events.Envelope{
		ID:         "event-123",
		Type:       "delivery.BuildCompleted",
		Version:    1,
		OccurredAt: time.Date(2026, 10, 17, 6, 0, 0, 0, time.UTC),
		Payload:    json.RawMessage(`{"build_id":"build-123"}`),
	}
}
