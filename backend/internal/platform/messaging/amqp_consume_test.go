package messaging

import (
	"context"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

func TestRetryFromHeadersSupportsAMQPIntegerTypes(t *testing.T) {
	require.Zero(t, retryFromHeaders(nil))
	require.Equal(t, 1, retryFromHeaders(amqp.Table{HeaderRetry: int32(1)}))
	require.Equal(t, 2, retryFromHeaders(amqp.Table{HeaderRetry: int64(2)}))
	require.Equal(t, 3, retryFromHeaders(amqp.Table{HeaderRetry: 3}))
	require.Zero(t, retryFromHeaders(amqp.Table{HeaderRetry: "3"}))
}

func TestHandleDeliveryDoesNotAckWhenRepublishFails(t *testing.T) {
	publishErr := errors.New("publish failed")
	acked := false

	err := handleDelivery(context.Background(), TopicDeploy, 0, []byte("body"),
		func(context.Context, []byte) error { return errors.New("handler failed") },
		func(context.Context, string, []byte, amqp.Table) error { return publishErr },
		func() error { acked = true; return nil })

	require.ErrorIs(t, err, publishErr)
	require.False(t, acked)
}

func TestHandleDeliveryAcksOnlyAfterRepublishSucceeds(t *testing.T) {
	published := false
	acked := false

	err := handleDelivery(context.Background(), TopicDeploy, 0, []byte("body"),
		func(context.Context, []byte) error { return errors.New("handler failed") },
		func(_ context.Context, topic string, _ []byte, headers amqp.Table) error {
			require.Equal(t, TopicDeploy, topic)
			require.Equal(t, int32(1), headers[HeaderRetry])
			published = true
			return nil
		},
		func() error {
			require.True(t, published)
			acked = true
			return nil
		})

	require.NoError(t, err)
	require.True(t, acked)
}

func TestHandleDeliveryDoesNotAckWhenDeadLetterPublishFails(t *testing.T) {
	publishErr := errors.New("dlq failed")
	acked := false

	err := handleDelivery(context.Background(), TopicDeploy, MaxConsumeRetry, []byte("body"),
		func(context.Context, []byte) error { return errors.New("handler failed") },
		func(_ context.Context, topic string, _ []byte, _ amqp.Table) error {
			require.Equal(t, DeadLetterTopic(TopicDeploy), topic)
			return publishErr
		},
		func() error { acked = true; return nil })

	require.ErrorIs(t, err, publishErr)
	require.False(t, acked)
}

func TestConsumeTopologyDeclaresDurableMainAndDeadLetterQueues(t *testing.T) {
	topology := consumeTopology(TopicDeploy)
	require.Equal(t, []queueBinding{
		{Name: TopicDeploy, RoutingKey: TopicDeploy, Durable: true},
		{Name: DeadLetterTopic(TopicDeploy), RoutingKey: DeadLetterTopic(TopicDeploy), Durable: true},
	}, topology)
}

type recordingTopologyChannel struct {
	declared []string
	durable  []bool
	bound    []string
}

func (c *recordingTopologyChannel) QueueDeclare(name string, durable, _, _, _ bool, _ amqp.Table) (amqp.Queue, error) {
	c.declared = append(c.declared, name)
	c.durable = append(c.durable, durable)
	return amqp.Queue{Name: name}, nil
}

func (c *recordingTopologyChannel) QueueBind(name, key, exchange string, _ bool, _ amqp.Table) error {
	c.bound = append(c.bound, name+"|"+key+"|"+exchange)
	return nil
}

func TestEnsureTopologyDeclaresAndBindsMainAndDeadLetter(t *testing.T) {
	channel := &recordingTopologyChannel{}

	require.NoError(t, ensureTopology(channel, "sparkops", TopicDeploy))

	require.Equal(t, []string{TopicDeploy, DeadLetterTopic(TopicDeploy)}, channel.declared)
	require.Equal(t, []bool{true, true}, channel.durable)
	require.Equal(t, []string{
		TopicDeploy + "|" + TopicDeploy + "|sparkops",
		DeadLetterTopic(TopicDeploy) + "|" + DeadLetterTopic(TopicDeploy) + "|sparkops",
	}, channel.bound)
}

type recordingConsumerResource struct {
	cancelled bool
	closed    bool
}

func (r *recordingConsumerResource) Cancel(string, bool) error {
	r.cancelled = true
	return nil
}

func (r *recordingConsumerResource) Close() error {
	r.closed = true
	return nil
}

func TestStopConsumerChannelOnlyClosesConsumerResource(t *testing.T) {
	resource := &recordingConsumerResource{}
	sharedConnectionClosed := false

	require.NoError(t, stopConsumerChannel(resource, "consumer-tag"))

	require.True(t, resource.cancelled)
	require.True(t, resource.closed)
	require.False(t, sharedConnectionClosed)
}
