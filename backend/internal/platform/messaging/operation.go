package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OperationRequest struct {
	Owner          string
	OperationType  string
	IdempotencyKey string
	Payload        any
	Repeatable     bool
}

type OperationWriter interface {
	Schedule(ctx context.Context, request OperationRequest) error
}

type Operation struct {
	ID             string
	EventID        string
	Owner          string
	OperationType  string
	IdempotencyKey string
	Payload        json.RawMessage
	Attempt        int
}

type OperationHandler func(ctx context.Context, operation Operation) error

type OperationDescriptor struct {
	Type    string
	Handler OperationHandler
}

type OperationRegistry interface {
	Register(operationType string, handler OperationHandler) error
	Handler(operationType string) (OperationHandler, bool)
}

func RegisterOperationDescriptors(registry OperationRegistry, descriptors []OperationDescriptor) error {
	if registry == nil {
		return fmt.Errorf("operation registry is required")
	}
	for _, descriptor := range descriptors {
		if err := registry.Register(descriptor.Type, descriptor.Handler); err != nil {
			return fmt.Errorf("register operation %s: %w", descriptor.Type, err)
		}
	}
	return nil
}

type operationWriter struct {
	store persistence.OperationStore
	clock Clock
}

func NewOperationWriter(store persistence.OperationStore, clock Clock) OperationWriter {
	if clock == nil {
		clock = systemClock{}
	}
	return &operationWriter{store: store, clock: clock}
}

func (w *operationWriter) Schedule(ctx context.Context, request OperationRequest) error {
	if _, err := postgres.RequireTx(ctx); err != nil {
		return err
	}
	if w.store == nil {
		return fmt.Errorf("operation store is required")
	}
	if request.OperationType == "" {
		return fmt.Errorf("operation type is required")
	}
	if request.IdempotencyKey == "" {
		return fmt.Errorf("operation idempotency key is required")
	}

	owner := request.Owner
	if owner == "" {
		return fmt.Errorf("operation owner is required")
	}
	keyEventID, err := validateOperationIdempotencyKey(
		request.IdempotencyKey,
		owner,
		request.OperationType,
	)
	if err != nil {
		return err
	}
	operationID := uuid.NewString()
	eventID := operationID
	if metadata, ok := events.MetadataFromContext(ctx); ok {
		if metadata.EventID == "" || metadata.Subscriber == "" {
			return fmt.Errorf("event metadata requires event id and subscriber")
		}
		if owner != metadata.Subscriber {
			return fmt.Errorf(
				"operation owner %q does not match event subscriber %q",
				owner,
				metadata.Subscriber,
			)
		}
		if keyEventID != metadata.EventID {
			return fmt.Errorf("operation idempotency key event id does not match event metadata")
		}
		eventID = metadata.EventID
	}

	payload, err := json.Marshal(request.Payload)
	if err != nil {
		return fmt.Errorf("marshal operation payload: %w", err)
	}
	now := w.clock.Now()
	err = w.store.Insert(ctx, persistence.OperationRecord{
		ID:             operationID,
		EventID:        eventID,
		Subscriber:     owner,
		OperationType:  request.OperationType,
		IdempotencyKey: request.IdempotencyKey,
		Payload:        payload,
		Repeatable:     request.Repeatable,
		NextAttemptAt:  now,
		CreatedAt:      now,
	})
	if err != nil {
		return fmt.Errorf("insert operation outbox: %w", err)
	}
	return nil
}

func validateOperationIdempotencyKey(
	key string,
	owner string,
	operationType string,
) (string, error) {
	parts := strings.SplitN(key, ":", 4)
	if len(parts) != 4 {
		return "", fmt.Errorf(
			"operation idempotency key must contain event id, owner, operation type, and target id",
		)
	}
	for _, part := range parts {
		if part == "" {
			return "", fmt.Errorf("operation idempotency key segments must be non-empty")
		}
	}
	if parts[1] != owner {
		return "", fmt.Errorf("operation idempotency key owner does not match operation owner")
	}
	if parts[2] != operationType {
		return "", fmt.Errorf("operation idempotency key type does not match operation type")
	}
	return parts[0], nil
}

type operationRegistry struct {
	mu       sync.RWMutex
	handlers map[string]OperationHandler
}

func NewOperationRegistry() OperationRegistry {
	return &operationRegistry{handlers: make(map[string]OperationHandler)}
}

func (r *operationRegistry) Register(operationType string, handler OperationHandler) error {
	if operationType == "" {
		return fmt.Errorf("operation type is required")
	}
	if handler == nil {
		return fmt.Errorf("operation handler is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[operationType]; exists {
		return fmt.Errorf("duplicate operation type %q", operationType)
	}
	r.handlers[operationType] = handler
	return nil
}

func (r *operationRegistry) Handler(operationType string) (OperationHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[operationType]
	return handler, ok
}
