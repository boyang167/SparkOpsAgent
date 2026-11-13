package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSearchIndex struct {
	ids []uuid.UUID
}

func (f fakeSearchIndex) Search(context.Context, SearchFiles) ([]uuid.UUID, string, error) {
	return append([]uuid.UUID{}, f.ids...), "", nil
}

type fakeSearchAuthorizer struct {
	allowed map[uuid.UUID]struct{}
}

func (f fakeSearchAuthorizer) Allow(_ context.Context, _, _, _, nodeID uuid.UUID) bool {
	_, ok := f.allowed[nodeID]
	return ok
}

func TestSearchReauthorizesEveryResult(t *testing.T) {
	fileA := uuid.New()
	fileB := uuid.New()
	handler := NewSearchFilesHandler(
		fakeSearchIndex{ids: []uuid.UUID{fileA, fileB}},
		nil,
		fakeSearchAuthorizer{allowed: map[uuid.UUID]struct{}{fileA: {}}},
	)
	results, _, err := handler.Handle(context.Background(), SearchFiles{
		ActorID: uuid.New(), WorkspaceID: uuid.New(), NamespaceID: uuid.New(), Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{fileA}, resultIDs(results))
}

func resultIDs(hits []SearchHit) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.NodeID)
	}
	return out
}
