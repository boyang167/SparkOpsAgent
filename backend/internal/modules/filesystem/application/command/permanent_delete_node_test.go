package command

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

func TestPermanentDeleteNodePurgesTrashedSubtree(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	bus := newFilesystemOutboxBus(t, db)
	tx := postgres.NewTransactor(db)
	source, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	require.NoError(t, repo.TrashNode(context.Background(), source.ID, time.Now().UTC(), source.OwnerID))
	handler := NewPermanentDeleteNodeHandler(repo, tx, bus, nil)
	cmd := PermanentDeleteNode{
		CommandID: uuid.New(), ActorID: source.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: source.ID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: source.ID, Permission: model.ResourcePermissionDelete,
		},
	}
	require.NoError(t, handler.Handle(context.Background(), cmd))
	purged, err := repo.GetNodeIncludingTrashed(context.Background(), ns.ID, source.ID)
	require.NoError(t, err)
	require.Equal(t, model.NodeStatePurged, purged.State)
	var quota struct {
		UsedBytes int64
		UsedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Equal(t, int64(0), quota.UsedBytes)
	require.Equal(t, int64(0), quota.UsedFiles)
}

func TestPermanentDeleteNodeReplayIsIdempotent(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	bus := newFilesystemOutboxBus(t, db)
	tx := postgres.NewTransactor(db)
	source, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	require.NoError(t, repo.TrashNode(context.Background(), source.ID, time.Now().UTC(), source.OwnerID))
	handler := NewPermanentDeleteNodeHandler(repo, tx, bus, nil)
	cmd := PermanentDeleteNode{
		CommandID: uuid.New(), ActorID: source.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: source.ID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: source.ID, Permission: model.ResourcePermissionDelete,
		},
	}
	require.NoError(t, handler.Handle(context.Background(), cmd))
	require.NoError(t, handler.Handle(context.Background(), cmd))
	var count int64
	require.NoError(t, db.Table("platform.event_outbox").Where("event_type = ?", filesystemevent.NodePurgedName).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestPermanentDeleteNodeRejectsNonTrashedState(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	tx := postgres.NewTransactor(db)
	source, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	handler := NewPermanentDeleteNodeHandler(repo, tx, nil, nil)
	err = handler.Handle(context.Background(), PermanentDeleteNode{
		CommandID: uuid.New(), ActorID: source.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: source.ID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: source.ID, Permission: model.ResourcePermissionDelete,
		},
	})
	require.ErrorIs(t, err, domain.ErrNodeStateConflict)
}