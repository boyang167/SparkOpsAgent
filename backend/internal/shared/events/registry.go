package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type Metadata struct {
	EventID       string
	EventType     string
	EventVersion  int
	Subscriber    string
	CorrelationID string
}

type metadataContextKey struct{}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	return context.WithValue(ctx, metadataContextKey{}, metadata)
}

func MetadataFromContext(ctx context.Context) (Metadata, bool) {
	metadata, ok := ctx.Value(metadataContextKey{}).(Metadata)
	return metadata, ok
}

type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

type descriptorKey struct {
	name    string
	version int
}

type descriptor struct {
	name    string
	version int
	encode  func(Event) ([]byte, AggregateRef, error)
	decode  func([]byte) (Event, error)
}

type Registry struct {
	mu          sync.RWMutex
	descriptors map[descriptorKey]descriptor
	encoders    map[string]descriptor
}

func NewRegistry() *Registry {
	return &Registry{
		descriptors: make(map[descriptorKey]descriptor),
		encoders:    make(map[string]descriptor),
	}
}

func RegisterJSON[T Event](
	registry *Registry,
	name string,
	version int,
	aggregate func(T) AggregateRef,
) error {
	if registry == nil {
		return fmt.Errorf("event registry is nil")
	}
	key := descriptorKey{name: name, version: version}
	entry := descriptor{
		name:    name,
		version: version,
		encode: func(event Event) ([]byte, AggregateRef, error) {
			value, ok := event.(T)
			if !ok {
				return nil, AggregateRef{}, fmt.Errorf("event %q has unexpected type %T", name, event)
			}
			payload, err := json.Marshal(value)
			if err != nil {
				return nil, AggregateRef{}, fmt.Errorf("marshal event %q: %w", name, err)
			}
			return payload, aggregate(value), nil
		},
		decode: func(payload []byte) (Event, error) {
			var value T
			if err := json.Unmarshal(payload, &value); err != nil {
				return nil, err
			}
			return value, nil
		},
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.descriptors[key]; exists {
		return fmt.Errorf("duplicate event descriptor %q version %d", name, version)
	}
	registry.descriptors[key] = entry
	if current, exists := registry.encoders[name]; !exists || version > current.version {
		registry.encoders[name] = entry
	}
	return nil
}

func (r *Registry) Encode(ctx context.Context, event Event) (Envelope, error) {
	r.mu.RLock()
	entry, ok := r.encoders[event.EventName()]
	r.mu.RUnlock()
	if !ok {
		return Envelope{}, fmt.Errorf("event descriptor not registered for %q", event.EventName())
	}
	payload, aggregate, err := entry.encode(event)
	if err != nil {
		return Envelope{}, err
	}

	id := uuid.NewString()
	correlationID := id
	causationID := ""
	if parent, ok := MetadataFromContext(ctx); ok {
		causationID = parent.EventID
		if parent.CorrelationID != "" {
			correlationID = parent.CorrelationID
		} else if parent.EventID != "" {
			correlationID = parent.EventID
		}
	}
	return Envelope{
		ID:               id,
		Type:             entry.name,
		Version:          entry.version,
		AggregateType:    aggregate.Type,
		AggregateID:      aggregate.ID,
		AggregateVersion: aggregate.Version,
		OccurredAt:       event.OccurredAt(),
		CorrelationID:    correlationID,
		CausationID:      causationID,
		Payload:          payload,
	}, nil
}

func (r *Registry) Decode(envelope Envelope) (Event, error) {
	r.mu.RLock()
	entry, ok := r.descriptors[descriptorKey{name: envelope.Type, version: envelope.Version}]
	r.mu.RUnlock()
	if !ok {
		return nil, &PermanentError{Err: fmt.Errorf(
			"unknown event type %q version %d",
			envelope.Type,
			envelope.Version,
		)}
	}
	event, err := entry.decode(envelope.Payload)
	if err != nil {
		return nil, &PermanentError{Err: fmt.Errorf(
			"decode event type %q version %d: %w",
			envelope.Type,
			envelope.Version,
			err,
		)}
	}
	return event, nil
}
