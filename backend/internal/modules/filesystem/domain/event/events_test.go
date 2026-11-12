package event

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

func TestRegisterEncodesAndDecodesVersionedFileEvents(t *testing.T) {
	registry := events.NewRegistry()
	require.NoError(t, Register(registry))

	fileID := uuid.New()
	created := FileCreated{
		BaseEvent:   events.BaseEvent{Name: FileCreatedName, At: time.Unix(10, 0).UTC()},
		FileID:      fileID,
		NamespaceID: uuid.New(),
	}
	envelope, err := registry.Encode(t.Context(), created)
	require.NoError(t, err)
	require.Equal(t, FileCreatedName, envelope.Type)
	require.Equal(t, 1, envelope.Version)
	require.Equal(t, "file", envelope.AggregateType)
	require.Equal(t, fileID.String(), envelope.AggregateID)

	decoded, err := registry.Decode(envelope)
	require.NoError(t, err)
	got, ok := decoded.(FileCreated)
	require.True(t, ok)
	require.Equal(t, created, got)

	revision := FileRevisionCreated{
		BaseEvent:  events.BaseEvent{Name: FileRevisionCreatedName, At: time.Unix(11, 0).UTC()},
		FileID:     fileID,
		RevisionID: uuid.New(),
		RevisionNo: 2,
	}
	revisionEnvelope, err := registry.Encode(t.Context(), revision)
	require.NoError(t, err)
	require.Equal(t, FileRevisionCreatedName, revisionEnvelope.Type)
	require.Equal(t, 1, revisionEnvelope.Version)
	decodedRevision, err := registry.Decode(revisionEnvelope)
	require.NoError(t, err)
	require.Equal(t, revision, decodedRevision)
}
