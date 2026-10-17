package messaging

import (
	"context"
	"encoding/json"
	"errors"
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

type OperationRegistry interface {
	Register(operationType string, handler OperationHandler) error
	Handler(operationType string) (OperationHandler, bool)
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

	id := uuid.NewString()
	eventID := id
	owner := request.Owner
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
		expectedPrefix := metadata.EventID + ":" + owner + ":" + request.OperationType + ":"
		if !strings.HasPrefix(request.IdempotencyKey, expectedPrefix) ||
			len(request.IdempotencyKey) == len(expectedPrefix) {
			return fmt.Errorf("operation idempotency key must match event, owner, type, and target")
		}
		eventID = metadata.EventID
	} else if owner == "" {
		return fmt.Errorf("operation owner is required without event metadata")
	}

	payload, err := json.Marshal(request.Payload)
	if err != nil {
		return fmt.Errorf("marshal operation payload: %w", err)
	}
	now := w.clock.Now()
	err = w.store.Insert(ctx, persistence.OperationRecord{
		ID:             id,
		EventID:        eventID,
		Subscriber:     owner,
		OperationType:  request.OperationType,
		IdempotencyKey: request.IdempotencyKey,
		Payload:        payload,
		NextAttemptAt:  now,
		CreatedAt:      now,
	})
	if errors.Is(err, persistence.ErrOperationIdempotencyConflict) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("insert operation outbox: %w", err)
	}
	return nil
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
