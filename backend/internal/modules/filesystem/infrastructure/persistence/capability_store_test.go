package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func TestCapabilityStoreKeepsHashAndRevocation(t *testing.T) {
	db := openCapabilityDB(t)
	store := NewCapabilityRepository(db)
	record := model.CapabilityRecord{
		ID: uuid.New(), TokenHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		OrganizationID: uuid.New(), WorkspaceID: uuid.New(), ActorID: uuid.New(), RunID: uuid.New(),
		ResourceIDs: []uuid.UUID{uuid.New()}, Actions: []string{"file:read"}, Audience: "sparkops-agent-capability",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	require.NoError(t, store.SaveCapability(context.Background(), record))
	loaded, err := store.GetCapabilityByHash(context.Background(), record.TokenHash)
	require.NoError(t, err)
	require.Equal(t, record.ResourceIDs, loaded.ResourceIDs)
	require.Nil(t, loaded.RevokedAt)
	require.NoError(t, store.RevokeCapability(context.Background(), record.ID, time.Now().UTC()))
	require.NoError(t, store.RevokeCapability(context.Background(), record.ID, time.Now().UTC()))
	revoked, err := store.GetCapability(context.Background(), record.ID)
	require.NoError(t, err)
	require.NotNil(t, revoked.RevokedAt)

	task := model.ExportTask{
		ID: uuid.New(), SourceOrganizationID: uuid.New(), TargetOrganizationID: uuid.New(),
		SourceWorkspaceID: uuid.New(), TargetWorkspaceID: uuid.New(), SourceNodeID: uuid.New(),
		Status: model.ExportStatusPending, ActorID: uuid.New(),
	}
	require.NoError(t, store.SaveExportTask(context.Background(), task))
	target := uuid.New()
	require.NoError(t, store.MarkExportImported(context.Background(), task.ID, target))
	imported, err := store.GetExportTask(context.Background(), task.ID)
	require.NoError(t, err)
	require.Equal(t, target, *imported.TargetNodeID)
	require.Equal(t, model.ExportStatusImported, imported.Status)
	_, err = store.GetCapability(context.Background(), uuid.New())
	require.ErrorIs(t, err, domain.ErrCapabilityNotFound)
}

func openCapabilityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	return db
}
