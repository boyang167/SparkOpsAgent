package messaging

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestFactoryStartFailureRollsBackInReverseOrder(t *testing.T) {
	var calls []string
	eventRuntime := &recordingLifecycle{
		start: func(context.Context) error {
			calls = append(calls, "event-start")
			return nil
		},
		close: func() error {
			calls = append(calls, "event-close")
			return nil
		},
	}
	operationRuntime := &recordingLifecycle{
		start: func(context.Context) error {
			calls = append(calls, "operation-start")
			return errors.New("operation start failed")
		},
		close: func() error {
			calls = append(calls, "operation-close")
			return nil
		},
	}
	queue := &recordingQueue{
		close: func() error {
			calls = append(calls, "queue-close")
			return nil
		},
	}
	factory := &Factory{
		Queue:          queue,
		eventLifecycle: eventRuntime,
		Operations:     &OperationRuntime{dispatcher: operationRuntime},
	}

	err := factory.Start(context.Background())
	require.ErrorContains(t, err, "operation start failed")
	require.Equal(t, []string{
		"event-start",
		"operation-start",
		"operation-close",
		"event-close",
		"queue-close",
	}, calls)
	require.NoError(t, factory.Close())
	require.Len(t, calls, 5)
}

func TestFactoryConcurrentCloseWaitsAndReturnsSameError(t *testing.T) {
	closeStarted := make(chan struct{})
	releaseClose := make(chan struct{})
	closeErr := errors.New("drain failed")
	var closeCalls int
	factory := &Factory{
		Operations: &OperationRuntime{dispatcher: &recordingLifecycle{
			close: func() error {
				closeCalls++
				close(closeStarted)
				<-releaseClose
				return closeErr
			},
		}},
	}

	results := make(chan error, 2)
	go func() { results <- factory.Close() }()
	<-closeStarted
	secondStarted := make(chan struct{})
	go func() {
		close(secondStarted)
		results <- factory.Close()
	}()
	<-secondStarted
	select {
	case err := <-results:
		require.Failf(t, "close returned before drain completed", "error: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseClose)

	first := <-results
	second := <-results
	require.ErrorIs(t, first, closeErr)
	require.ErrorIs(t, second, closeErr)
	require.Equal(t, 1, closeCalls)
}

func TestFactoryStopsAllIntakeBeforeWaitingAndClosingResources(t *testing.T) {
	var calls []string
	eventRuntime := &stagedRecordingLifecycle{
		recordingLifecycle: recordingLifecycle{
			close: func() error {
				calls = append(calls, "event-close")
				return nil
			},
		},
		stop: func() { calls = append(calls, "event-stop") },
		wait: func() { calls = append(calls, "event-wait") },
	}
	operationRuntime := &stagedRecordingLifecycle{
		recordingLifecycle: recordingLifecycle{
			close: func() error {
				calls = append(calls, "operation-close")
				return nil
			},
		},
		stop: func() { calls = append(calls, "operation-stop") },
		wait: func() { calls = append(calls, "operation-wait") },
	}
	factory := &Factory{
		eventLifecycle: eventRuntime,
		Operations:     &OperationRuntime{dispatcher: operationRuntime},
		Queue: &recordingQueue{close: func() error {
			calls = append(calls, "queue-close")
			return nil
		}},
	}

	require.NoError(t, factory.Close())
	require.Equal(t, []string{
		"operation-stop",
		"event-stop",
		"operation-wait",
		"event-wait",
		"operation-close",
		"event-close",
		"queue-close",
	}, calls)
}

func TestFactoryAMQPAssemblyUsesSeparateEventTransportAndStableRuntimeIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)

	var queueConfig AMQPConfig
	var eventURL string
	var eventExchange string
	queue := &recordingQueue{close: func() error { return nil }}
	transport := &fakeEventTransport{}
	cfg := &config.Config{Messaging: config.MessagingConfig{
		Driver: "amqp",
		AMQP: config.AMQPConfig{
			URL:           "amqp://broker",
			Exchange:      "task.exchange",
			ExchangeType:  "direct",
			EventExchange: "event.exchange",
		},
		Outbox: config.OutboxConfig{
			PollInterval: time.Second,
			BatchSize:    42,
			LockTimeout:  time.Minute,
		},
	}}

	factory, err := newFromConfig(
		cfg,
		db,
		events.NewRegistry(),
		postgres.NewTransactor(db),
		zap.NewNop(),
		factoryDependencies{
			newQueue: func(cfg AMQPConfig) (Queue, error) {
				queueConfig = cfg
				return queue, nil
			},
			newMetrics: func(prometheus.Registerer) (*Metrics, error) {
				return metrics, nil
			},
			newEventTransport: func(url string, _ *Metrics, exchanges ...string) (EventTransport, error) {
				eventURL = url
				eventExchange = exchanges[0]
				return transport, nil
			},
			runtimeIdentity: func() (string, string) {
				return "instance-a", "runtime-a"
			},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, factory.Close()) })
	require.Equal(t, "task.exchange", queueConfig.Exchange)
	require.Equal(t, "amqp://broker", eventURL)
	require.Equal(t, "event.exchange", eventExchange)

	bus := factory.Bus.(*ReliableBus)
	dispatcher := bus.dispatcher.(*OutboxDispatcher)
	require.Equal(t, "instance-a.runtime-a.event-outbox", dispatcher.config.Owner)
	require.Equal(t, 42, dispatcher.config.BatchSize)
	operationDispatcher := factory.Operations.dispatcher.(*OperationDispatcher)
	require.Equal(t, "instance-a", operationDispatcher.config.Owner)
	require.Equal(t, "runtime-a", operationDispatcher.runtimeID)
}

func TestFactoryConstructionFailureClosesAlreadyOpenedQueue(t *testing.T) {
	closeErr := errors.New("queue close failed")
	queue := &recordingQueue{close: func() error { return closeErr }}
	transportErr := errors.New("event transport failed")
	cfg := &config.Config{Messaging: config.MessagingConfig{
		Driver: "amqp",
		AMQP:   config.AMQPConfig{URL: "amqp://broker", EventExchange: "events"},
	}}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	_, err = newFromConfig(
		cfg,
		db,
		events.NewRegistry(),
		postgres.NewTransactor(db),
		zap.NewNop(),
		factoryDependencies{
			newQueue: func(AMQPConfig) (Queue, error) { return queue, nil },
			newMetrics: func(prometheus.Registerer) (*Metrics, error) {
				return NewMetrics(prometheus.NewRegistry())
			},
			newEventTransport: func(string, *Metrics, ...string) (EventTransport, error) {
				return nil, transportErr
			},
			runtimeIdentity: runtimeIdentity,
		},
	)
	require.ErrorIs(t, err, transportErr)
	require.ErrorIs(t, err, closeErr)
}

type recordingLifecycle struct {
	start func(context.Context) error
	close func() error
}

type stagedRecordingLifecycle struct {
	recordingLifecycle
	stop func()
	wait func()
}

func (l *stagedRecordingLifecycle) StopAccepting() {
	l.stop()
}

func (l *stagedRecordingLifecycle) Wait() {
	l.wait()
}

func (l *recordingLifecycle) Start(ctx context.Context) error {
	if l.start == nil {
		return nil
	}
	return l.start(ctx)
}

func (l *recordingLifecycle) Close() error {
	if l.close == nil {
		return nil
	}
	return l.close()
}

type recordingQueue struct {
	mu    sync.Mutex
	close func() error
}

func (*recordingQueue) Publish(context.Context, string, []byte) error { return nil }

func (*recordingQueue) Subscribe(context.Context, string, func(context.Context, []byte) error) error {
	return nil
}

func (q *recordingQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.close()
}
