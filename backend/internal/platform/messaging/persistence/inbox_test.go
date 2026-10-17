package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/stretchr/testify/require"
)

func TestInboxTryInsertRequiresTransaction(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewInboxStore(db)

	_, err := store.TryInsert(context.Background(), uuid.NewString(), "notification.build-completed", "delivery.BuildCompleted", time.Now().UTC())

	require.ErrorContains(t, err, "transaction required")
}

func TestInboxTryInsertDeduplicatesEventPerSubscriber(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewInboxStore(db)
	transactor := postgres.NewTransactor(db)
	eventID := uuid.NewString()
	now := time.Now().UTC()

	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		inserted, err := store.TryInsert(ctx, eventID, "notification.build-completed", "delivery.BuildCompleted", now)
		require.NoError(t, err)
		require.True(t, inserted)

		inserted, err = store.TryInsert(ctx, eventID, "notification.build-completed", "delivery.BuildCompleted", now)
		require.NoError(t, err)
		require.False(t, inserted)
		return nil
	}))
}

func TestInboxRollsBackWithBusinessWrite(t *testing.T) {
	db := openStoreSQLite(t)
	require.NoError(t, db.Exec("CREATE TABLE platform.task3_business_rows (id TEXT PRIMARY KEY)").Error)
	store := NewInboxStore(db)
	transactor := postgres.NewTransactor(db)
	eventID := uuid.NewString()
	businessID := uuid.NewString()
	rollback := errors.New("rollback")

	err := transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		tx, err := postgres.RequireTx(ctx)
		require.NoError(t, err)
		require.NoError(t, tx.Exec("INSERT INTO platform.task3_business_rows(id) VALUES (?)", businessID).Error)
		inserted, err := store.TryInsert(ctx, eventID, "notification.build-completed", "delivery.BuildCompleted", time.Now().UTC())
		require.NoError(t, err)
		require.True(t, inserted)
		return rollback
	})

	require.ErrorIs(t, err, rollback)
	var inboxCount, businessCount int64
	require.NoError(t, db.Model(&EventInbox{}).Where("event_id = ?", eventID).Count(&inboxCount).Error)
	require.NoError(t, db.Table("platform.task3_business_rows").Where("id = ?", businessID).Count(&businessCount).Error)
	require.Zero(t, inboxCount)
	require.Zero(t, businessCount)
}
