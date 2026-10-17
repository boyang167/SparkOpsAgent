package messaging

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Factory struct {
	Bus        Bus
	Queue      Queue
	Operations *OperationRuntime

	eventLifecycle Lifecycle
	mu             sync.Mutex
	started        bool
	closed         bool
	closeErr       error
	closeDone      chan struct{}
}

type OperationRuntime struct {
	Registry OperationRegistry
	Writer   OperationWriter

	dispatcher DispatcherLifecycle
}

type factoryDependencies struct {
	newQueue          func(AMQPConfig) (Queue, error)
	newMetrics        func(prometheus.Registerer) (*Metrics, error)
	newEventTransport func(string, *Metrics, ...string) (EventTransport, error)
	runtimeIdentity   func() (string, string)
}

func (r *OperationRuntime) Start(ctx context.Context) error {
	if r == nil || r.dispatcher == nil {
		return nil
	}
	return r.dispatcher.Start(ctx)
}

func (r *OperationRuntime) Close() error {
	if r == nil || r.dispatcher == nil {
		return nil
	}
	return r.dispatcher.Close()
}

func (r *OperationRuntime) StopAccepting() {
	if r == nil || r.dispatcher == nil {
		return
	}
	if staged, ok := r.dispatcher.(interface{ StopAccepting() }); ok {
		staged.StopAccepting()
	}
}

func (r *OperationRuntime) Wait() {
	if r == nil || r.dispatcher == nil {
		return
	}
	if staged, ok := r.dispatcher.(interface{ Wait() }); ok {
		staged.Wait()
	}
}

func NewFromConfig(
	cfg *config.Config,
	db *gorm.DB,
	registry *events.Registry,
	transactor postgres.Transactor,
	logger *zap.Logger,
) (*Factory, error) {
	return newFromConfig(
		cfg,
		db,
		registry,
		transactor,
		logger,
		factoryDependencies{
			newQueue:          NewAMQPQueue,
			newMetrics:        NewMetrics,
			newEventTransport: NewAMQPEventTransport,
			runtimeIdentity:   runtimeIdentity,
		},
	)
}

func newFromConfig(
	cfg *config.Config,
	db *gorm.DB,
	registry *events.Registry,
	transactor postgres.Transactor,
	logger *zap.Logger,
	dependencies factoryDependencies,
) (*Factory, error) {
	if cfg == nil {
		return nil, fmt.Errorf("messaging config is required")
	}
	if registry == nil {
		return nil, fmt.Errorf("event registry is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	driver := cfg.Messaging.Driver
	if driver == "" {
		driver = "inmemory"
	}
	driver = strings.ToLower(strings.TrimSpace(driver))
	instanceOwner, runtimeID := dependencies.runtimeIdentity()

	var metrics *Metrics
	if driver == "amqp" || db != nil {
		var err error
		metrics, err = dependencies.newMetrics(prometheus.DefaultRegisterer)
		if err != nil {
			return nil, fmt.Errorf("create messaging metrics: %w", err)
		}
	}

	factory := &Factory{}
	switch driver {
	case "inmemory":
		factory.Bus = NewInMemoryBus(registry)
		factory.Queue = NewNoopQueue()
	case "amqp":
		if db == nil {
			return nil, fmt.Errorf("database is required when messaging.driver=amqp")
		}
		if transactor == nil {
			return nil, fmt.Errorf("transactor is required when messaging.driver=amqp")
		}
		queue, err := dependencies.newQueue(AMQPConfig{
			URL:          cfg.Messaging.AMQP.URL,
			Exchange:     cfg.Messaging.AMQP.Exchange,
			ExchangeType: cfg.Messaging.AMQP.ExchangeType,
		})
		if err != nil {
			return nil, fmt.Errorf("create amqp queue: %w", err)
		}
		factory.Queue = queue

		transport, err := dependencies.newEventTransport(
			cfg.Messaging.AMQP.URL,
			metrics,
			cfg.Messaging.AMQP.EventExchange,
		)
		if err != nil {
			return nil, errors.Join(
				fmt.Errorf("create amqp event transport: %w", err),
				queue.Close(),
			)
		}

		outbox := persistence.NewOutboxStore(db)
		inbox := persistence.NewInboxStore(db)
		dispatcher := NewOutboxDispatcher(
			outbox,
			transport,
			OutboxDispatcherConfig{
				Owner:        boundedRuntimeOwner(instanceOwner, runtimeID, "event-outbox"),
				BatchSize:    cfg.Messaging.Outbox.BatchSize,
				Lease:        cfg.Messaging.Outbox.LockTimeout,
				PollInterval: cfg.Messaging.Outbox.PollInterval,
			},
			nil,
			metrics,
		)
		consumer := NewInboxConsumer(transactor, inbox, registry, transport, nil, metrics)
		reliableBus := NewReliableBus(registry, outbox, transport, dispatcher, consumer)
		factory.Bus = reliableBus
		factory.eventLifecycle = reliableBus
		logger.Info(
			"reliable event runtime configured",
			zap.String("instance_owner", instanceOwner),
			zap.String("runtime_id", runtimeID),
		)
	default:
		return nil, fmt.Errorf("unknown messaging driver: %s", driver)
	}

	if db != nil {
		operationStore := persistence.NewOperationStore(db)
		operationRegistry := NewOperationRegistry()
		factory.Operations = &OperationRuntime{
			Registry: operationRegistry,
			Writer:   NewOperationWriter(operationStore, nil),
			dispatcher: NewOperationDispatcher(
				operationStore,
				operationRegistry,
				OperationDispatcherConfig{
					Owner:        instanceOwner,
					RuntimeID:    runtimeID,
					BatchSize:    cfg.Messaging.Outbox.BatchSize,
					Lease:        cfg.Messaging.Outbox.LockTimeout,
					PollInterval: cfg.Messaging.Outbox.PollInterval,
				},
				nil,
				metrics,
			),
		}
	}
	return factory, nil
}

func (f *Factory) Start(ctx context.Context) error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return fmt.Errorf("messaging factory is closed")
	}
	if f.started {
		f.mu.Unlock()
		return nil
	}
	f.mu.Unlock()

	if f.eventLifecycle != nil {
		if err := f.eventLifecycle.Start(ctx); err != nil {
			return f.rollbackStart(fmt.Errorf("start event runtime: %w", err))
		}
	}
	if f.Operations != nil {
		if err := f.Operations.Start(ctx); err != nil {
			return f.rollbackStart(fmt.Errorf("start operation runtime: %w", err))
		}
	}

	f.mu.Lock()
	f.started = true
	f.mu.Unlock()
	return nil
}

func (f *Factory) rollbackStart(startErr error) error {
	closeErr := f.close()
	return errors.Join(startErr, closeErr)
}

func (f *Factory) Close() error {
	return f.close()
}

func (f *Factory) close() error {
	f.mu.Lock()
	if f.closed {
		done := f.closeDone
		f.mu.Unlock()
		if done != nil {
			<-done
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.closeErr
	}
	f.closed = true
	f.closeDone = make(chan struct{})
	f.mu.Unlock()

	var closeErrors []error
	if f.Operations != nil {
		f.Operations.StopAccepting()
	}
	if staged, ok := f.eventLifecycle.(interface{ StopAccepting() }); ok {
		staged.StopAccepting()
	}
	if f.Operations != nil {
		f.Operations.Wait()
	}
	if staged, ok := f.eventLifecycle.(interface{ Wait() }); ok {
		staged.Wait()
	}
	if f.Operations != nil {
		closeErrors = append(closeErrors, f.Operations.Close())
	}
	if f.eventLifecycle != nil {
		closeErrors = append(closeErrors, f.eventLifecycle.Close())
	}
	if f.Queue != nil {
		closeErrors = append(closeErrors, f.Queue.Close())
	}
	closeErr := errors.Join(closeErrors...)

	f.mu.Lock()
	f.closeErr = closeErr
	close(f.closeDone)
	f.mu.Unlock()
	return closeErr
}

func runtimeIdentity() (string, string) {
	instanceOwner := strings.TrimSpace(os.Getenv("SPARKOPS_INSTANCE_ID"))
	if instanceOwner == "" {
		instanceOwner, _ = os.Hostname()
	}
	if instanceOwner == "" {
		instanceOwner = "sparkops"
	}
	return instanceOwner, uuid.NewString()
}

func boundedRuntimeOwner(instanceOwner, runtimeID, role string) string {
	full := fmt.Sprintf("%s.%s.%s", instanceOwner, runtimeID, role)
	if len(full) <= operationOwnerMaxLength {
		return full
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(full)))[:16]
	suffix := "." + digest + "." + role
	maxPrefix := operationOwnerMaxLength - len(suffix)
	if maxPrefix < 0 {
		return digest
	}
	return string([]rune(instanceOwner)[:min(len([]rune(instanceOwner)), maxPrefix)]) + suffix
}
