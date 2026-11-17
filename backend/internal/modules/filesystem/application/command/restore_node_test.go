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

func TestTrashNodeStoresDeletionMetadata(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	tx := postgres.NewTransactor(db)
	actor := uuid.New()
	node, err := model.NewDirectory(ns.ID, actor, "trash-meta", *ns.RootNodeID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), node))
	logs := &recordingAudit{}
	handler := NewTrashNodeHandler(repo, tx, nil, logs)
	cmd := TrashNode{
		CommandID: uuid.New(), ActorID: actor, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: node.ID,
		Authorization: Authorization{Allowed: true},
	}
	require.NoError(t, handler.Handle(context.Background(), cmd))
	persisted, err := repo.GetNodeIncludingTrashed(context.Background(), ns.ID, node.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted.DeletedAt)
	require.NotNil(t, persisted.DeletedBy)
	require.Equal(t, actor, *persisted.DeletedBy)
}

func TestRestoreNodeRejectsParentWriteAuthorization(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	tx := postgres.NewTransactor(db)
	actor := uuid.New()
	file, err := model.NewFile(ns.ID, actor, "restore-deny.txt", *ns.RootNodeID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), file))
	require.NoError(t, repo.TrashNode(context.Background(), file.ID, time.Now().UTC(), actor))
	handler := NewRestoreNodeHandler(repo, tx, nil, nil)
	parentID := *ns.RootNodeID
	err = handler.Handle(context.Background(), RestoreNode{
		CommandID: uuid.New(), ActorID: actor, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: file.ID,
		Authorization: Authorization{
			Allowed: false, Reason: "default_deny",
			WorkspaceID: ns.WorkspaceID, ResourceID: parentID, Permission: model.ResourcePermissionWrite,
		},
	})
	require.ErrorIs(t, err, domain.ErrResourceDenied)
}

func TestRestoreNodeRejectsNameConflict(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	tx := postgres.NewTransactor(db)
	trashed, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	require.NoError(t, repo.TrashNode(context.Background(), trashed.ID, time.Now().UTC(), trashed.OwnerID))
	_, err = NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	handler := NewRestoreNodeHandler(repo, tx, nil, nil)
	parentID := *ns.RootNodeID
	err = handler.Handle(context.Background(), RestoreNode{
		CommandID: uuid.New(), ActorID: trashed.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: trashed.ID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: parentID, Permission: model.ResourcePermissionWrite,
		},
	})
	require.ErrorIs(t, err, domain.ErrRestoreNameConflict)
}

func TestRestoreNodeReplayPublishesEventOnce(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	bus := newFilesystemOutboxBus(t, db)
	tx := postgres.NewTransactor(db)
	source, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	other := putFixture(ns)
	other.Name = "other-before-trash.txt"
	other.CommandID = uuid.New()
	_, err = NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), other)
	require.NoError(t, err)
	require.NoError(t, repo.TrashNode(context.Background(), source.ID, time.Now().UTC(), source.OwnerID))
	handler := NewRestoreNodeHandler(repo, tx, bus, nil)
	parentID := *ns.RootNodeID
	cmd := RestoreNode{
		CommandID: uuid.New(), ActorID: source.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: source.ID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: parentID, Permission: model.ResourcePermissionWrite,
		},
	}
	require.NoError(t, handler.Handle(context.Background(), cmd))
	require.NoError(t, handler.Handle(context.Background(), cmd))
	var count int64
	require.NoError(t, db.Table("platform.event_outbox").Where("event_type = ?", filesystemevent.NodeRestoredName).Count(&count).Error)
	require.Equal(t, int64(1), count)
	restored, err := repo.GetNode(context.Background(), ns.ID, source.ID)
	require.NoError(t, err)
	require.Equal(t, model.NodeStateActive, restored.State)
}