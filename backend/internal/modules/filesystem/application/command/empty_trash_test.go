package command

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

func TestEmptyTrashReturnsPerItemResultsAndReplayIsStable(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	bus := newFilesystemOutboxBus(t, db)
	tx := postgres.NewTransactor(db)
	first, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	secondFixture := putFixture(ns)
	secondFixture.Name = "empty-trash-b.txt"
	secondFixture.CommandID = uuid.New()
	second, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), secondFixture)
	require.NoError(t, err)
	require.NoError(t, repo.TrashNode(context.Background(), first.ID, time.Now().UTC(), first.OwnerID))
	require.NoError(t, repo.TrashNode(context.Background(), second.ID, time.Now().UTC(), second.OwnerID))
	handler := NewEmptyTrashHandler(repo, tx, bus, nil)
	auth := Authorization{Allowed: true, WorkspaceID: ns.WorkspaceID}
	batchID := uuid.New()
	result, err := handler.Handle(context.Background(), EmptyTrash{
		CommandID: batchID, ActorID: first.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, Authorization: auth,
	})
	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	successes := 0
	for _, item := range result.Items {
		if item.Error == nil {
			successes++
		}
	}
	require.Equal(t, 2, successes)
	result, err = handler.Handle(context.Background(), EmptyTrash{
		CommandID: batchID, ActorID: first.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, Authorization: auth,
	})
	require.NoError(t, err)
	require.Empty(t, result.Items)
}

func TestEmptyTrashPartialSuccessKeepsFailedEntries(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	tx := postgres.NewTransactor(db)
	good, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	parent, err := model.NewDirectory(ns.ID, good.OwnerID, "hold-parent", *ns.RootNodeID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), parent))
	child, err := model.NewFile(ns.ID, good.OwnerID, "hold-child.txt", parent.ID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), child))
	require.NoError(t, repo.TrashNode(context.Background(), parent.ID, time.Now().UTC(), good.OwnerID))
	require.NoError(t, repo.TrashNode(context.Background(), good.ID, time.Now().UTC(), good.OwnerID))
	require.NoError(t, db.Exec("UPDATE filesystem.nodes SET state = ? WHERE id = ?", model.NodeStateRetentionHold, child.ID).Error)
	handler := NewEmptyTrashHandler(repo, tx, nil, nil)
	auth := Authorization{Allowed: true, WorkspaceID: ns.WorkspaceID}
	result, err := handler.Handle(context.Background(), EmptyTrash{
		CommandID: uuid.New(), ActorID: good.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, Authorization: auth,
	})
	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	var sawRetention bool
	var sawSuccess bool
	for _, item := range result.Items {
		switch item.NodeID {
		case good.ID:
			require.NoError(t, item.Error)
			sawSuccess = true
		case parent.ID:
			require.ErrorIs(t, item.Error, domain.ErrRetentionHold)
			sawRetention = true
		}
	}
	require.True(t, sawRetention)
	require.True(t, sawSuccess)
}