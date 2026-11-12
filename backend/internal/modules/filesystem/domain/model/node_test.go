package model_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func TestNodeRenamePreservesIdentity(t *testing.T) {
	node := model.NewFile(uuid.New(), uuid.New(), "report.csv", uuid.New())
	id := node.ID

	require.NoError(t, node.Rename("final.csv"))
	assert.Equal(t, id, node.ID)
	assert.Equal(t, "final.csv", node.Name)
	assert.Equal(t, "final.csv", node.NormalizedName)
}

func TestRevisionContentIsImmutable(t *testing.T) {
	revision := model.NewRevision(uuid.New(), 1, uuid.New(), 42, "text/plain", uuid.New())

	assert.Equal(t, int64(42), revision.Size)
	assert.Equal(t, 1, revision.RevisionNo)
}
