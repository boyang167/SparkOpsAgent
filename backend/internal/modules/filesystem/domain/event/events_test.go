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

func TestRegisterCoversEveryG1MutationEvent(t *testing.T) {
	require.Equal(t, "filesystem.file.moved.v1", NodeMovedName)
	require.Equal(t, "filesystem.file.trashed.v1", NodeTrashedName)
	registry := events.NewRegistry()
	require.NoError(t, Register(registry))
	nodeID := uuid.New()
	for name, event := range map[string]events.Event{
		DirectoryCreatedName:  DirectoryCreated{BaseEvent: events.NewBaseEvent(DirectoryCreatedName), NodeID: nodeID},
		NodeRenamedName:       NodeRenamed{BaseEvent: events.NewBaseEvent(NodeRenamedName), NodeID: nodeID},
		NodeMovedName:         NodeMoved{BaseEvent: events.NewBaseEvent(NodeMovedName), NodeID: nodeID},
		NodeCopiedName:        NodeCopied{BaseEvent: events.NewBaseEvent(NodeCopiedName), NodeID: nodeID},
		NodeTrashedName:       NodeTrashed{BaseEvent: events.NewBaseEvent(NodeTrashedName), NodeID: nodeID},
		RevisionRestoredName:  RevisionRestored{BaseEvent: events.NewBaseEvent(RevisionRestoredName), NodeID: nodeID},
		PermissionChangedName: PermissionChanged{BaseEvent: events.NewBaseEvent(PermissionChangedName), NodeID: nodeID},
	} {
		envelope, err := registry.Encode(t.Context(), event)
		require.NoError(t, err, name)
		require.Equal(t, name, envelope.Type)
		require.Equal(t, 1, envelope.Version)
		require.Equal(t, nodeID.String(), envelope.AggregateID)
		_, err = registry.Decode(envelope)
		require.NoError(t, err, name)
	}
}

func TestRegisterEncodesTransferInitiatedV1(t *testing.T) {
	registry := events.NewRegistry()
	require.NoError(t, Register(registry))
	sessionID := uuid.New()
	ev := TransferInitiated{
		BaseEvent: events.NewBaseEvent(TransferInitiatedName),
		SessionID: sessionID, WorkspaceID: uuid.New(), NamespaceID: uuid.New(),
	}

	envelope, err := registry.Encode(t.Context(), ev)

	require.NoError(t, err)
	require.Equal(t, "filesystem.transfer.initiated.v1", envelope.Type)
	require.Equal(t, 1, envelope.Version)
	require.Equal(t, "transfer", envelope.AggregateType)
	require.Equal(t, sessionID.String(), envelope.AggregateID)
	decoded, err := registry.Decode(envelope)
	require.NoError(t, err)
	require.Equal(t, ev, decoded)
}
