package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestRegistryRoundTripsConcreteValueEvent(t *testing.T) {
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON[deliveryevent.BuildCompleted](
		registry,
		deliveryevent.BuildCompletedName,
		1,
		func(ev deliveryevent.BuildCompleted) events.AggregateRef {
			return events.AggregateRef{Type: "build", ID: ev.BuildID.String()}
		},
	))
	event := deliveryevent.NewBuildCompleted(
		kernel.BuildID{UUID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		kernel.ProjectID{UUID: uuid.MustParse("22222222-2222-2222-2222-222222222222")},
		kernel.MemberID{UUID: uuid.MustParse("33333333-3333-3333-3333-333333333333")},
		kernel.WorkspaceID{UUID: uuid.MustParse("44444444-4444-4444-4444-444444444444")},
		"orders",
		"succeeded",
		42,
	)

	envelope, err := registry.Encode(context.Background(), event)
	require.NoError(t, err)
	require.NotEmpty(t, envelope.ID)
	require.Equal(t, deliveryevent.BuildCompletedName, envelope.Type)
	require.Equal(t, 1, envelope.Version)
	require.Equal(t, "build", envelope.AggregateType)
	require.Equal(t, event.BuildID.String(), envelope.AggregateID)

	decoded, err := registry.Decode(envelope)
	require.NoError(t, err)
	require.IsType(t, deliveryevent.BuildCompleted{}, decoded)
	require.Equal(t, event.BuildID, decoded.(deliveryevent.BuildCompleted).BuildID)
}

func TestRegistryEncodePropagatesParentMetadata(t *testing.T) {
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON[deliveryevent.BuildCompleted](
		registry,
		deliveryevent.BuildCompletedName,
		1,
		func(ev deliveryevent.BuildCompleted) events.AggregateRef {
			return events.AggregateRef{Type: "build", ID: ev.BuildID.String()}
		},
	))
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID:       "parent-event",
		CorrelationID: "root-correlation",
	})

	envelope, err := registry.Encode(ctx, deliveryevent.NewBuildCompleted(
		kernel.NewBuildID(),
		kernel.NewProjectID(),
		kernel.NewMemberID(),
		kernel.NewWorkspaceID(),
		"orders",
		"succeeded",
		42,
	))

	require.NoError(t, err)
	require.Equal(t, "root-correlation", envelope.CorrelationID)
	require.Equal(t, "parent-event", envelope.CausationID)
}

func TestRegistryDecodeReturnsPermanentError(t *testing.T) {
	registry := events.NewRegistry()

	_, err := registry.Decode(events.Envelope{Type: "unknown", Version: 1})

	var permanent *events.PermanentError
	require.ErrorAs(t, err, &permanent)
}

func TestRegistryDecodeMalformedPayloadReturnsPermanentErrorWithDescriptorContext(t *testing.T) {
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON[deliveryevent.BuildCompleted](
		registry,
		deliveryevent.BuildCompletedName,
		1,
		func(ev deliveryevent.BuildCompleted) events.AggregateRef {
			return events.AggregateRef{Type: "build", ID: ev.BuildID.String()}
		},
	))

	_, err := registry.Decode(events.Envelope{
		Type:    deliveryevent.BuildCompletedName,
		Version: 1,
		Payload: []byte(`{"BuildID":`),
	})

	var permanent *events.PermanentError
	require.True(t, errors.As(err, &permanent))
	require.ErrorContains(t, err, deliveryevent.BuildCompletedName)
	require.ErrorContains(t, err, "version 1")
}

func TestRegistryRejectsDuplicateDescriptor(t *testing.T) {
	registry := events.NewRegistry()
	register := func() error {
		return events.RegisterJSON[deliveryevent.BuildCompleted](
			registry,
			deliveryevent.BuildCompletedName,
			1,
			func(ev deliveryevent.BuildCompleted) events.AggregateRef {
				return events.AggregateRef{Type: "build", ID: ev.BuildID.String()}
			},
		)
	}

	require.NoError(t, register())
	require.ErrorContains(t, register(), "duplicate")
}
