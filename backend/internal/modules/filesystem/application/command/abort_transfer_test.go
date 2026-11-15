package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
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

func TestAbortTransferWithoutWorkspaceReleasesNamespaceQuota(t *testing.T) {
	for _, namespaceType := range []model.NamespaceType{
		model.NamespaceTypeOrganization,
		model.NamespaceTypeUser,
	} {
		t.Run(string(namespaceType), func(t *testing.T) {
			ctx := context.Background()
			db, workspaceNamespace := newTransferTestDB(t)
			repo := persistence.NewRepository(db)
			namespace := &model.Namespace{
				ID: uuid.New(), NamespaceType: namespaceType,
				OrganizationID: workspaceNamespace.OrganizationID, Status: "ACTIVE",
			}
			if namespaceType == model.NamespaceTypeUser {
				namespace.OwnerUserID = uuid.New()
			}
			require.NoError(t, repo.CreateNamespace(ctx, namespace))
			root, err := model.NewRootDirectory(namespace.ID, uuid.New(), ".root")
			require.NoError(t, err)
			require.NoError(t, repo.CreateNode(ctx, root))
			require.NoError(t, repo.SetNamespaceRootByID(ctx, namespace.ID, root.ID))
			namespace.RootNodeID = &root.ID

			create := newTransferHandler(db)
			cmd := transferFixture(workspaceNamespace)
			cmd.WorkspaceID = uuid.Nil
			cmd.NamespaceID = namespace.ID
			cmd.ParentID = namespace.RootNodeID
			cmd.Authorization.WorkspaceID = uuid.Nil
			cmd.Authorization.ResourceID = root.ID
			cmd.IdempotencyKey = "namespace-abort-" + string(namespaceType)
			descriptor, err := create.Handle(ctx, cmd)
			require.NoError(t, err)

			abort := NewAbortTransferHandler(repo, postgres.NewTransactor(db))
			require.NoError(t, abort.Handle(ctx, AbortTransfer{
				NamespaceID: namespace.ID, SessionID: descriptor.SessionID, ActorID: cmd.ActorID,
				Authorization: Authorization{
					Allowed: true, ResourceID: root.ID, Permission: model.ResourcePermissionWrite,
				},
			}))

			var reservation struct {
				State       string
				WorkspaceID *uuid.UUID
			}
			require.NoError(t, db.Table("filesystem.quota_reservations").
				Where("session_id = ?", descriptor.SessionID).Take(&reservation).Error)
			require.Equal(t, "RELEASED", reservation.State)
			require.Nil(t, reservation.WorkspaceID)
			var account struct{ ReservedBytes, ReservedFiles int64 }
			require.NoError(t, db.Table("filesystem.quota_accounts").
				Where("namespace_id = ?", namespace.ID).Take(&account).Error)
			require.Zero(t, account.ReservedBytes)
			require.Zero(t, account.ReservedFiles)
		})
	}
}

func TestAbortTransferAllowsUserNamespaceWithWorkspaceRoute(t *testing.T) {
	ctx := context.Background()
	db, workspaceNamespace := newTransferTestDB(t)
	repo := persistence.NewRepository(db)
	userNS := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeUser,
		OrganizationID: workspaceNamespace.OrganizationID, OwnerUserID: uuid.New(), Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(ctx, userNS))
	root, err := model.NewRootDirectory(userNS.ID, uuid.New(), ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(ctx, root))
	require.NoError(t, repo.SetNamespaceRootByID(ctx, userNS.ID, root.ID))
	userNS.RootNodeID = &root.ID

	create := newTransferHandler(db)
	cmd := transferFixture(workspaceNamespace)
	cmd.NamespaceID = userNS.ID
	cmd.ParentID = userNS.RootNodeID
	cmd.Authorization.WorkspaceID = workspaceNamespace.WorkspaceID
	cmd.Authorization.ResourceID = root.ID
	cmd.IdempotencyKey = "user-ns-workspace-abort"
	descriptor, err := create.Handle(ctx, cmd)
	require.NoError(t, err)

	abort := NewAbortTransferHandler(repo, postgres.NewTransactor(db))
	require.NoError(t, abort.Handle(ctx, AbortTransfer{
		WorkspaceID: workspaceNamespace.WorkspaceID, NamespaceID: userNS.ID,
		SessionID: descriptor.SessionID, ActorID: cmd.ActorID,
		Authorization: Authorization{
			Allowed: true, WorkspaceID: workspaceNamespace.WorkspaceID, ResourceID: root.ID,
			Permission: model.ResourcePermissionWrite,
		},
	}))
	session, err := repo.GetTransferByID(ctx, descriptor.SessionID)
	require.NoError(t, err)
	require.Equal(t, model.TransferAborted, session.Status())
}

func TestAbortTransferRejectsWrongNamespaceAndWorkspace(t *testing.T) {
	ctx := context.Background()
	db, namespace := newTransferTestDB(t)
	create := newTransferHandler(db)
	cmd := transferFixture(namespace)
	descriptor, err := create.Handle(ctx, cmd)
	require.NoError(t, err)
	abort := NewAbortTransferHandler(persistence.NewRepository(db), postgres.NewTransactor(db))
	auth := Authorization{
		Allowed: true, WorkspaceID: namespace.WorkspaceID, ResourceID: *cmd.ParentID,
		Permission: model.ResourcePermissionWrite,
	}

	err = abort.Handle(ctx, AbortTransfer{
		NamespaceID: uuid.New(), WorkspaceID: namespace.WorkspaceID,
		SessionID: descriptor.SessionID, ActorID: cmd.ActorID, Authorization: auth,
	})
	require.ErrorIs(t, err, domain.ErrTransferNotFound)
	err = abort.Handle(ctx, AbortTransfer{
		NamespaceID: namespace.ID, WorkspaceID: uuid.New(),
		SessionID: descriptor.SessionID, ActorID: cmd.ActorID, Authorization: auth,
	})
	require.ErrorIs(t, err, domain.ErrTransferNotFound)

	var reservation struct{ State string }
	require.NoError(t, db.Table("filesystem.quota_reservations").
		Where("session_id = ?", descriptor.SessionID).Take(&reservation).Error)
	require.Equal(t, "RESERVED", reservation.State)
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
