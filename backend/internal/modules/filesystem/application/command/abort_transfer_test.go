package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func TestAbortTransferReleasesQuotaReservation(t *testing.T) {
	ctx := context.Background()
	db, ns := newTransferTestDB(t)
	handler := newTransferHandler(db)
	cmd := transferFixture(ns)
	descriptor, err := handler.Handle(ctx, cmd)
	require.NoError(t, err)

	var before struct {
		ReservedBytes int64
		ReservedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).
		Select("reserved_bytes", "reserved_files").Take(&before).Error)
	require.Equal(t, int64(60), before.ReservedBytes)

	abort := NewAbortTransferHandler(persistence.NewRepository(db), postgres.NewTransactor(db))
	require.NoError(t, abort.Handle(ctx, AbortTransfer{
		WorkspaceID: ns.WorkspaceID,
		SessionID:   descriptor.SessionID,
		ActorID:     cmd.ActorID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: *cmd.ParentID,
			Permission: model.ResourcePermissionWrite,
		},
	}))

	var after struct {
		ReservedBytes int64
		ReservedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).
		Select("reserved_bytes", "reserved_files").Take(&after).Error)
	require.Zero(t, after.ReservedBytes)
	require.Zero(t, after.ReservedFiles)

	var reservation struct{ State string }
	require.NoError(t, db.Table("filesystem.quota_reservations").Where("session_id = ?", descriptor.SessionID).Take(&reservation).Error)
	require.Equal(t, "RELEASED", reservation.State)

	session, err := persistence.NewRepository(db).GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	require.Equal(t, model.TransferAborted, session.Status())
}

func TestAbortTransferIsIdempotentWhenAlreadyAborted(t *testing.T) {
	ctx := context.Background()
	db, ns := newTransferTestDB(t)
	create := newTransferHandler(db)
	cmd := transferFixture(ns)
	descriptor, err := create.Handle(ctx, cmd)
	require.NoError(t, err)
	abort := NewAbortTransferHandler(persistence.NewRepository(db), postgres.NewTransactor(db))
	request := AbortTransfer{
		WorkspaceID: ns.WorkspaceID, SessionID: descriptor.SessionID, ActorID: cmd.ActorID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: *cmd.ParentID,
			Permission: model.ResourcePermissionWrite,
		},
	}
	require.NoError(t, abort.Handle(ctx, request))
	require.NoError(t, abort.Handle(ctx, request))
}

func TestAbortTransferRejectsCompletedSession(t *testing.T) {
	ctx := context.Background()
	db, ns := newTransferTestDB(t)
	repo := persistence.NewRepository(db)
	handler := newTransferHandler(db)
	cmd := transferFixture(ns)
	descriptor, err := handler.Handle(ctx, cmd)
	require.NoError(t, err)
	session, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	require.NoError(t, repo.SaveTransferStateCAS(ctx, ns.WorkspaceID, session.ID(), session.Version(), repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated,
		StatusTo:   model.TransferCompleted,
	}))

	abort := NewAbortTransferHandler(repo, postgres.NewTransactor(db))
	err = abort.Handle(ctx, AbortTransfer{
		WorkspaceID: ns.WorkspaceID, SessionID: descriptor.SessionID, ActorID: cmd.ActorID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: *cmd.ParentID,
			Permission: model.ResourcePermissionWrite,
		},
	})
	require.ErrorIs(t, err, domain.ErrTransferStatusConflict)
}
