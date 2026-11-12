package model_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func TestNodeRenamePreservesIdentity(t *testing.T) {
	node, err := model.NewFile(uuid.New(), uuid.New(), "report.csv", uuid.New())
	require.NoError(t, err)
	id := node.ID

	require.NoError(t, node.Rename("  Final.csv  "))
	assert.Equal(t, id, node.ID)
	assert.Equal(t, "Final.csv", node.Name)
	assert.Equal(t, "final.csv", node.NormalizedName)
}

func TestNodeConstructorRejectsInvalidName(t *testing.T) {
	_, err := model.NewFile(uuid.New(), uuid.New(), "bad/name", uuid.New())

	require.ErrorIs(t, err, model.ErrInvalidNodeName)
}

func TestNodeStatesIncludeRetentionLifecycle(t *testing.T) {
	assert.Equal(t, model.NodeState("ACTIVE"), model.NodeStateActive)
	assert.Equal(t, model.NodeState("TRASHED"), model.NodeStateTrashed)
	assert.Equal(t, model.NodeState("RETENTION_HOLD"), model.NodeStateRetentionHold)
	assert.Equal(t, model.NodeState("PURGE_PENDING"), model.NodeStatePurgePending)
	assert.Equal(t, model.NodeState("PURGED"), model.NodeStatePurged)
}

func TestRevisionContentIsImmutable(t *testing.T) {
	revision, err := model.NewRevision(uuid.New(), 1, uuid.New(), 42, "text/plain", uuid.New())
	require.NoError(t, err)

	assert.Equal(t, int64(42), revision.Size())
	assert.Equal(t, 1, revision.RevisionNo())
	assert.NotPanics(t, func() {
		_ = revision.BlobID()
		_ = revision.MIMEType()
	})
}

func TestRevisionRejectsInvalidContent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		revisionNo int
		size       int64
		mimeType   string
	}{
		{"zero revision", 0, 1, "text/plain"},
		{"negative revision", -1, 1, "text/plain"},
		{"negative size", 1, -1, "text/plain"},
		{"empty mime", 1, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := model.NewRevision(uuid.New(), tc.revisionNo, uuid.New(), tc.size, tc.mimeType, uuid.New())
			require.Error(t, err)
		})
	}
}
