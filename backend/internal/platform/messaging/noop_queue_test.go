package messaging

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopQueueRetriesThreeTimesThenPublishesDLQ(t *testing.T) {
	queue := NewNoopQueue()
	attempts := 0
	var deadLetter []byte
	require.NoError(t, queue.Subscribe(context.Background(), TopicDeploy, func(context.Context, []byte) error {
		attempts++
		return errors.New("failed")
	}))
	require.NoError(t, queue.Subscribe(context.Background(), DeadLetterTopic(TopicDeploy), func(_ context.Context, payload []byte) error {
		deadLetter = append([]byte(nil), payload...)
		return nil
	}))

	require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("payload")))

	require.Equal(t, MaxConsumeRetry+1, attempts)
	require.Equal(t, []byte("payload"), deadLetter)
}

func TestNoopQueueStopsRetryingAfterSuccess(t *testing.T) {
	queue := NewNoopQueue()
	attempts := 0
	require.NoError(t, queue.Subscribe(context.Background(), TopicDeploy, func(context.Context, []byte) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	}))

	require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("payload")))
	require.Equal(t, 3, attempts)
}

func TestNoopQueueDoesNotRetryDeadLetterHandler(t *testing.T) {
	queue := NewNoopQueue()
	deadLetterCalls, nestedCalls := 0, 0
	require.NoError(t, queue.Subscribe(context.Background(), TopicDeploy, func(context.Context, []byte) error {
		return errors.New("failed")
	}))
	require.NoError(t, queue.Subscribe(context.Background(), DeadLetterTopic(TopicDeploy), func(context.Context, []byte) error {
		deadLetterCalls++
		return errors.New("dlq unavailable")
	}))
	require.NoError(t, queue.Subscribe(context.Background(), DeadLetterTopic(DeadLetterTopic(TopicDeploy)), func(context.Context, []byte) error {
		nestedCalls++
		return nil
	}))

	require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("payload")))
	require.Equal(t, 1, deadLetterCalls)
	require.Zero(t, nestedCalls)
}

func TestNoopQueueDeliversPublishBeforeSubscribeExactlyOnce(t *testing.T) {
	queue := NewNoopQueue()
	require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("first")))
	require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("second")))
	var received []string

	require.NoError(t, queue.Subscribe(context.Background(), TopicDeploy, func(_ context.Context, payload []byte) error {
		received = append(received, string(payload))
		return nil
	}))

	require.Equal(t, []string{"first", "second"}, received)
}

func TestNoopQueueReturnsErrorWhenPendingCapacityIsFull(t *testing.T) {
	queue := NewNoopQueue()
	for index := 0; index < NoopPendingCapacity; index++ {
		require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("payload")))
	}

	err := queue.Publish(context.Background(), TopicDeploy, []byte("overflow"))

	require.ErrorIs(t, err, ErrNoopQueueFull)
}

func TestNoopQueueDrainFailurePreservesCurrentAndRemainingMessages(t *testing.T) {
	queue := NewNoopQueue()
	require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("current")))
	require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("remaining")))

	entered := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	subscribeDone := make(chan error, 1)
	go func() {
		subscribeDone <- queue.Subscribe(context.Background(), TopicDeploy, func(context.Context, []byte) error {
			first.Do(func() {
				close(entered)
				<-release
			})
			return errors.New("handler failed")
		})
	}()
	<-entered

	for {
		err := queue.Publish(context.Background(), TopicBuild, []byte("capacity filler"))
		if errors.Is(err, ErrNoopQueueFull) {
			break
		}
		require.NoError(t, err)
	}
	close(release)
	require.ErrorIs(t, <-subscribeDone, ErrNoopQueueFull)

	require.NoError(t, queue.Subscribe(context.Background(), TopicBuild, func(context.Context, []byte) error {
		return nil
	}))
	var received []string
	require.NoError(t, queue.Subscribe(context.Background(), TopicDeploy, func(_ context.Context, payload []byte) error {
		received = append(received, string(payload))
		return nil
	}))
	require.Equal(t, []string{"current", "remaining"}, received)
}

func TestNoopQueueDefersSameTopicPublishUntilCurrentHandlerReturns(t *testing.T) {
	queue := NewNoopQueue()
	var order []string
	depth, maxDepth := 0, 0
	require.NoError(t, queue.Subscribe(context.Background(), TopicDeploy, func(ctx context.Context, payload []byte) error {
		depth++
		if depth > maxDepth {
			maxDepth = depth
		}
		order = append(order, "start-"+string(payload))
		if string(payload) == "first" {
			require.NoError(t, queue.Publish(ctx, TopicDeploy, []byte("second")))
		}
		order = append(order, "end-"+string(payload))
		depth--
		return nil
	}))

	require.NoError(t, queue.Publish(context.Background(), TopicDeploy, []byte("first")))

	require.Equal(t, 1, maxDepth)
	require.Equal(t, []string{"start-first", "end-first", "start-second", "end-second"}, order)
}
