package query

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type listTrashEntriesRepo struct {
	last repository.TrashListQuery
}

func (r *listTrashEntriesRepo) ListTrashEntries(_ context.Context, q repository.TrashListQuery) ([]repository.TrashEntry, string, error) {
	r.last = q
	return []repository.TrashEntry{{
		Node: model.Node{ID: uuid.New(), State: model.NodeStateTrashed},
	}}, "", nil
}

func TestListTrashClampsLimitAndForwardsQuery(t *testing.T) {
	repo := &listTrashEntriesRepo{}
	handler := NewListTrashHandler(repo)
	nsID := uuid.New()
	_, err := handler.Handle(context.Background(), ListTrash{NamespaceID: nsID, Limit: 0, Cursor: "cursor-a"})
	require.NoError(t, err)
	require.Equal(t, nsID, repo.last.NamespaceID)
	require.Equal(t, 100, repo.last.Limit)
	require.Equal(t, "cursor-a", repo.last.Cursor)

	_, err = handler.Handle(context.Background(), ListTrash{NamespaceID: nsID, Limit: 500})
	require.NoError(t, err)
	require.Equal(t, 200, repo.last.Limit)
}

func TestListTrashReturnsRepositoryEntries(t *testing.T) {
	deletedAt := time.Now().UTC()
	entry := repository.TrashEntry{
		Node: model.Node{ID: uuid.New(), Name: "trash.txt", State: model.NodeStateTrashed},
		OriginalPath: "/trash.txt", DeletedAt: deletedAt, DeletedBy: uuid.New(),
	}
	repo := &listTrashEntriesRepoReturning{entries: []repository.TrashEntry{entry}}
	result, err := NewListTrashHandler(repo).Handle(context.Background(), ListTrash{NamespaceID: uuid.New(), Limit: 10})
	require.NoError(t, err)
	require.Equal(t, entry, result.Items[0])
}

type listTrashEntriesRepoReturning struct {
	entries []repository.TrashEntry
}

func (r *listTrashEntriesRepoReturning) ListTrashEntries(context.Context, repository.TrashListQuery) ([]repository.TrashEntry, string, error) {
	return r.entries, "", nil
}