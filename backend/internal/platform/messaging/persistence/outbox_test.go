package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOutboxInsertRequiresTransaction(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOutboxStore(db)

	err := store.Insert(context.Background(), testEnvelope(time.Now().UTC()))

	require.ErrorContains(t, err, "transaction required")
}

func TestOutboxInsertRollsBackWithTransaction(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOutboxStore(db)
	transactor := postgres.NewTransactor(db)
	envelope := testEnvelope(time.Now().UTC())
	rollback := errors.New("rollback")

	err := transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NoError(t, store.Insert(ctx, envelope))
		return rollback
	})

	require.ErrorIs(t, err, rollback)
	var count int64
	require.NoError(t, db.Model(&EventOutbox{}).Where("id = ?", envelope.ID).Count(&count).Error)
	require.Zero(t, count)
}

func TestOutboxLifecycle(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOutboxStore(db)
	transactor := postgres.NewTransactor(db)
	envelope := testEnvelope(time.Now().UTC())
	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return store.Insert(ctx, envelope)
	}))

	const owner = "worker-a"
	setOutboxLease(t, db, envelope.ID, owner)
	next := envelope.OccurredAt.Add(time.Minute)
	require.NoError(t, store.Reschedule(context.Background(), envelope.ID, owner, "temporary", next))
	var row EventOutbox
	require.NoError(t, db.First(&row, "id = ?", envelope.ID).Error)
	require.Equal(t, "pending", row.Status)
	require.Equal(t, "temporary", row.LastError)
	require.WithinDuration(t, next, row.NextAttemptAt, time.Millisecond)

	setOutboxLease(t, db, envelope.ID, owner)
	publishedAt := next.Add(time.Minute)
	require.NoError(t, store.MarkPublished(context.Background(), envelope.ID, owner, publishedAt))
	require.NoError(t, db.First(&row, "id = ?", envelope.ID).Error)
	require.Equal(t, "published", row.Status)
	require.Empty(t, row.LastError)
	require.NotNil(t, row.PublishedAt)
	require.WithinDuration(t, publishedAt, *row.PublishedAt, time.Millisecond)
}

func TestOutboxCompletionRejectsLostLease(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOutboxStore(db)
	transactor := postgres.NewTransactor(db)
	envelope := testEnvelope(time.Now().UTC())
	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return store.Insert(ctx, envelope)
	}))
	setOutboxLease(t, db, envelope.ID, "new-owner")

	err := store.MarkPublished(context.Background(), envelope.ID, "old-owner", time.Now().UTC())

	require.ErrorIs(t, err, ErrLeaseLost)
	var row EventOutbox
	require.NoError(t, db.First(&row, "id = ?", envelope.ID).Error)
	require.Equal(t, "publishing", row.Status)
	require.Equal(t, "new-owner", row.LockedBy)
}

func TestOutboxUpdatesRejectMissingOrWrongState(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOutboxStore(db)
	missingID := uuid.NewString()

	require.ErrorIs(t,
		store.MarkPublished(context.Background(), missingID, "worker-a", time.Now().UTC()),
		ErrLeaseLost,
	)
	require.ErrorIs(t,
		store.Reschedule(context.Background(), missingID, "worker-a", "temporary", time.Now().UTC()),
		ErrLeaseLost,
	)

	envelope := testEnvelope(time.Now().UTC())
	transactor := postgres.NewTransactor(db)
	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return store.Insert(ctx, envelope)
	}))
	require.ErrorIs(t,
		store.Reschedule(context.Background(), envelope.ID, "worker-a", "temporary", time.Now().UTC()),
		ErrLeaseLost,
	)
}

func TestOutboxClaimEligibility(t *testing.T) {
	now := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	lease := 30 * time.Second
	activeLock := now.Add(-lease + time.Second)
	expiredLock := now.Add(-lease)

	tests := []struct {
		name          string
		status        string
		nextAttemptAt time.Time
		lockedAt      *time.Time
		want          bool
	}{
		{name: "pending due", status: "pending", nextAttemptAt: now, want: true},
		{name: "pending scheduled later", status: "pending", nextAttemptAt: now.Add(time.Second), want: false},
		{name: "publishing active lease", status: "publishing", lockedAt: &activeLock, want: false},
		{name: "publishing expired lease", status: "publishing", lockedAt: &expiredLock, want: true},
		{name: "published", status: "published", nextAttemptAt: now, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, outboxClaimable(tt.status, tt.nextAttemptAt, tt.lockedAt, now, lease))
		})
	}
}

func openStoreSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, AutoMigrate(db))
	return db
}

func testEnvelope(now time.Time) events.Envelope {
	return events.Envelope{
		ID:            uuid.NewString(),
		Type:          "delivery.BuildCompleted",
		Version:       1,
		AggregateType: "build",
		AggregateID:   uuid.NewString(),
		OccurredAt:    now,
		Payload:       []byte(`{"result":"succeeded"}`),
	}
}

func setOutboxLease(t *testing.T, db *gorm.DB, id, owner string) {
	t.Helper()
	require.NoError(t, db.Model(&EventOutbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":    "publishing",
		"locked_by": owner,
		"locked_at": time.Now().UTC(),
	}).Error)
}
