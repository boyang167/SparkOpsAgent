//go:build integration

package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

func TestAMQPQueueReconnectsPublisherAndConsumerAfterConnectionLoss(t *testing.T) {
	amqpURL := requireIntegrationEnv(t, "AMQP_URL")
	topic := TopicDeploy
	queueInterface, err := NewAMQPQueue(AMQPConfig{
		URL: amqpURL, Exchange: "sparkops", ExchangeType: "direct",
	})
	require.NoError(t, err)
	queue := queueInterface.(*amqpQueue)
	t.Cleanup(func() { _ = queue.Close() })
	cleanupAMQPTaskQueues(t, amqpURL, topic)

	received := make(chan string, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, queue.Subscribe(ctx, topic, func(_ context.Context, payload []byte) error {
		received <- string(payload)
		return nil
	}))

	queue.mu.Lock()
	failedConnection := queue.conn
	queue.mu.Unlock()
	require.NoError(t, failedConnection.Close())

	require.NoError(t, queue.Publish(ctx, topic, []byte("reconnected")))
	select {
	case payload := <-received:
		require.Equal(t, "reconnected", payload)
	case <-ctx.Done():
		t.Fatal("same queue instance did not restore its consumer")
	}

	queue.mu.Lock()
	reconnectedOnce := queue.conn
	queue.mu.Unlock()
	require.NotSame(t, failedConnection, reconnectedOnce)

	require.NoError(t, reconnectedOnce.Close())
	require.NoError(t, queue.Publish(ctx, topic, []byte("reconnected-twice")))
	select {
	case payload := <-received:
		require.Equal(t, "reconnected-twice", payload)
	case <-ctx.Done():
		t.Fatal("same queue instance did not restore its consumer after the second connection loss")
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	require.NotSame(t, reconnectedOnce, queue.conn)
	require.Contains(t, queue.subscriptions, topic)
}

func TestAMQPQueueAllowsResubscribeAfterContextCancellation(t *testing.T) {
	amqpURL := requireIntegrationEnv(t, "AMQP_URL")
	queueInterface, err := NewAMQPQueue(AMQPConfig{
		URL: amqpURL, Exchange: "sparkops", ExchangeType: "direct",
	})
	require.NoError(t, err)
	queue := queueInterface.(*amqpQueue)
	t.Cleanup(func() { _ = queue.Close() })
	cleanupAMQPTaskQueues(t, amqpURL, TopicDeploy)

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	require.NoError(t, queue.Subscribe(firstCtx, TopicDeploy, func(context.Context, []byte) error { return nil }))
	cancelFirst()
	require.Eventually(t, func() bool {
		queue.mu.Lock()
		defer queue.mu.Unlock()
		_, exists := queue.subscriptions[TopicDeploy]
		return !exists
	}, 2*time.Second, 10*time.Millisecond)

	secondCtx, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	require.NoError(t, queue.Subscribe(secondCtx, TopicDeploy, func(context.Context, []byte) error { return nil }))
}

func TestAMQPQueueTopologyFailureReturnsWithoutDeadlock(t *testing.T) {
	amqpURL := requireIntegrationEnv(t, "AMQP_URL")
	connection, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	channel, err := connection.Channel()
	require.NoError(t, err)
	_, err = channel.QueueDeclare(TopicDeploy, false, false, false, false, nil)
	require.NoError(t, err)
	require.NoError(t, channel.Close())
	require.NoError(t, connection.Close())
	t.Cleanup(func() {
		cleanupConnection, cleanupErr := amqp.Dial(amqpURL)
		if cleanupErr != nil {
			return
		}
		defer cleanupConnection.Close()
		cleanupChannel, cleanupErr := cleanupConnection.Channel()
		if cleanupErr != nil {
			return
		}
		defer cleanupChannel.Close()
		_, _ = cleanupChannel.QueueDelete(TopicDeploy, false, false, false)
		_, _ = cleanupChannel.QueueDelete(DeadLetterTopic(TopicDeploy), false, false, false)
	})

	queueInterface, err := NewAMQPQueue(AMQPConfig{
		URL: amqpURL, Exchange: "sparkops", ExchangeType: "direct",
	})
	require.NoError(t, err)
	queue := queueInterface.(*amqpQueue)

	done := make(chan error, 1)
	go func() {
		done <- queue.Subscribe(context.Background(), TopicDeploy, func(context.Context, []byte) error { return nil })
	}()
	select {
	case subscribeErr := <-done:
		require.Error(t, subscribeErr)
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe deadlocked after topology declaration failure")
	}
}

func TestAMQPQueueRecoversConsumerAfterDeliveryChannelFailure(t *testing.T) {
	amqpURL := requireIntegrationEnv(t, "AMQP_URL")
	queueInterface, err := NewAMQPQueue(AMQPConfig{
		URL: amqpURL, Exchange: "sparkops", ExchangeType: "direct",
	})
	require.NoError(t, err)
	queue := queueInterface.(*amqpQueue)
	t.Cleanup(func() { _ = queue.Close() })
	cleanupAMQPTaskQueues(t, amqpURL, TopicDeploy)

	entered := make(chan struct{})
	release := make(chan struct{})
	received := make(chan string, 1)
	first := true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, queue.Subscribe(ctx, TopicDeploy, func(_ context.Context, payload []byte) error {
		if first {
			first = false
			close(entered)
			<-release
			return errors.New("force retry publish after channel failure")
		}
		received <- string(payload)
		return nil
	}))
	require.NoError(t, queue.Publish(ctx, TopicDeploy, []byte("first")))
	<-entered

	queue.mu.Lock()
	var failedChannel *amqp.Channel
	for channel := range queue.consumerChannels {
		failedChannel = channel
		break
	}
	queue.mu.Unlock()
	require.NotNil(t, failedChannel)
	require.NoError(t, failedChannel.Close())
	close(release)

	require.Eventually(t, func() bool {
		queue.mu.Lock()
		defer queue.mu.Unlock()
		if len(queue.consumerChannels) != 1 {
			return false
		}
		for channel := range queue.consumerChannels {
			return channel != failedChannel
		}
		return false
	}, 5*time.Second, 20*time.Millisecond)

	require.NoError(t, queue.Publish(ctx, TopicDeploy, []byte("second")))
	for {
		select {
		case payload := <-received:
			if payload == "second" {
				return
			}
		case <-ctx.Done():
			t.Fatal("recovered consumer did not receive the next delivery")
		}
	}
}

func TestAMQPQueueRecoversIdleConsumerAfterChannelClosure(t *testing.T) {
	amqpURL := requireIntegrationEnv(t, "AMQP_URL")
	queueInterface, err := NewAMQPQueue(AMQPConfig{
		URL: amqpURL, Exchange: "sparkops", ExchangeType: "direct",
	})
	require.NoError(t, err)
	queue := queueInterface.(*amqpQueue)
	t.Cleanup(func() { _ = queue.Close() })
	cleanupAMQPTaskQueues(t, amqpURL, TopicDeploy)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, queue.Subscribe(ctx, TopicDeploy, func(context.Context, []byte) error { return nil }))

	queue.mu.Lock()
	var failedChannel *amqp.Channel
	for channel := range queue.consumerChannels {
		failedChannel = channel
		break
	}
	queue.mu.Unlock()
	require.NotNil(t, failedChannel)
	require.NoError(t, failedChannel.Close())

	require.Eventually(t, func() bool {
		queue.mu.Lock()
		defer queue.mu.Unlock()
		for channel := range queue.consumerChannels {
			if channel != failedChannel {
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond)
}

func cleanupAMQPTaskQueues(t *testing.T, amqpURL, topic string) {
	t.Helper()
	t.Cleanup(func() {
		connection, err := amqp.Dial(amqpURL)
		if err != nil {
			return
		}
		defer connection.Close()
		channel, err := connection.Channel()
		if err != nil {
			return
		}
		defer channel.Close()
		_, _ = channel.QueueDelete(topic, false, false, false)
		_, _ = channel.QueueDelete(DeadLetterTopic(topic), false, false, false)
	})
}
