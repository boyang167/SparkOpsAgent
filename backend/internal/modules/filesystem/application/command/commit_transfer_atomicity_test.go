package command

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"gorm.io/gorm"
)

func TestCommitTransferIsAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	db, repo, ns := newTransferCommitDB(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })

	providerID := uuid.New()
	content := []byte("transfer-bytes")
	checksum := storage.SHA256(content)
	cmd := transferFixture(ns)
	cmd.Size = int64(len(content))
	cmd.SHA256 = checksum
	cmd.ProviderID = providerID

	descriptor, err := newTransferHandler(db).Handle(ctx, cmd)
	require.NoError(t, err)

	session, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: session.TemporaryObjectKey(),
		Body:      bytes.NewReader(content),
		Size:      int64(len(content)),
		SHA256:    checksum,
	})
	require.NoError(t, err)

	require.NoError(t, repo.SaveTransferStateCAS(ctx, ns.WorkspaceID, session.ID(), session.Version(), repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferUploading,
	}))

	verify := NewVerifyTransferHandler(
		repo, postgres.NewTransactor(db),
		MapProviders{providerID: provider},
		port.NoopScanner{},
	)
	require.NoError(t, verify.Handle(ctx, VerifyTransfer{SessionID: descriptor.SessionID, WorkspaceID: ns.WorkspaceID}))

	bus := newTransferOutboxBus(t, db)
	commit := NewCommitTransferHandler(repo, postgres.NewTransactor(db), bus)

	first, err := commit.Handle(ctx, CommitTransfer{SessionID: descriptor.SessionID, WorkspaceID: ns.WorkspaceID})
	require.NoError(t, err)
	second, err := commit.Handle(ctx, CommitTransfer{SessionID: descriptor.SessionID, WorkspaceID: ns.WorkspaceID})
	require.NoError(t, err)
	require.Equal(t, first.RevisionID, second.RevisionID)
	require.Equal(t, first.NodeID, second.NodeID)
	node, err := repo.GetNodeByID(ctx, first.NodeID)
	require.NoError(t, err)
	require.Equal(t, cmd.Classification, node.Classification)
	var committedNodeID string
	require.NoError(t, db.Table("filesystem.transfer_sessions").Select("committed_node_id").
		Where("id = ?", descriptor.SessionID).Scan(&committedNodeID).Error)
	require.Equal(t, first.NodeID.String(), committedNodeID)

	assertQuotaCommittedOnce(t, db, ns.WorkspaceID, cmd.Size)
	assertOutboxEventCount(t, db, filesystemevent.TransferCompletedName, 1)
	assertOutboxEventCount(t, db, filesystemevent.FileCreatedName, 1)
	assertOutboxEventCount(t, db, filesystemevent.FileRevisionCreatedName, 1)
}

func newTransferOutboxBus(t *testing.T, db *gorm.DB) messaging.Bus {
	t.Helper()
	registry := sharedevents.NewRegistry()
	require.NoError(t, filesystemevent.Register(registry))
	return messaging.NewReliableBus(registry, messagingpersistence.NewOutboxStore(db), nil, nil, nil)
}

func newTransferCommitDB(t *testing.T) (*gorm.DB, repository.FilesystemRepository, *model.Namespace) {
	t.Helper()
	db, ns := newTransferTestDB(t)
	return db, persistence.NewRepository(db), ns
}

func assertQuotaCommittedOnce(t *testing.T, db *gorm.DB, workspaceID uuid.UUID, bytes int64) {
	t.Helper()
	var quota struct {
		UsedBytes, ReservedBytes int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", workspaceID).Take(&quota).Error)
	require.Equal(t, bytes, quota.UsedBytes)
	require.Zero(t, quota.ReservedBytes)
	var reservation struct{ State string }
	require.NoError(t, db.Table("filesystem.quota_reservations").Take(&reservation).Error)
	require.Equal(t, "COMMITTED", reservation.State)
}

func assertOutboxEventCount(t *testing.T, db *gorm.DB, eventType string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table("platform.event_outbox").Where("event_type = ?", eventType).Count(&count).Error)
	require.Equal(t, want, count, eventType)
}
