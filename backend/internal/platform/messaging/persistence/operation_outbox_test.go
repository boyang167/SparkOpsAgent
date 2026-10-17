package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/stretchr/testify/require"
)

func TestOperationInsertRequiresTransaction(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOperationStore(db)

	err := store.Insert(context.Background(), testOperation(time.Now().UTC()))

	require.ErrorContains(t, err, "transaction required")
}

func TestOperationLifecycle(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOperationStore(db)
	transactor := postgres.NewTransactor(db)
	operation := testOperation(time.Now().UTC())

	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return store.Insert(ctx, operation)
	}))

	next := operation.NextAttemptAt.Add(time.Minute)
	require.NoError(t, store.Reschedule(context.Background(), operation.ID, "temporary", next))
	var row OperationOutbox
	require.NoError(t, db.First(&row, "id = ?", operation.ID).Error)
	require.Equal(t, "pending", row.Status)
	require.Equal(t, "temporary", row.LastError)
	require.WithinDuration(t, next, row.NextAttemptAt, time.Millisecond)

	failedAt := next.Add(time.Minute)
	require.NoError(t, store.MarkFailed(context.Background(), operation.ID, "permanent", failedAt))
	require.NoError(t, db.First(&row, "id = ?", operation.ID).Error)
	require.Equal(t, "failed", row.Status)
	require.Equal(t, "permanent", row.LastError)
	require.NotNil(t, row.CompletedAt)
	require.WithinDuration(t, failedAt, *row.CompletedAt, time.Millisecond)

	completedAt := failedAt.Add(time.Minute)
	require.NoError(t, store.MarkCompleted(context.Background(), operation.ID, completedAt))
	require.NoError(t, db.First(&row, "id = ?", operation.ID).Error)
	require.Equal(t, "completed", row.Status)
	require.Empty(t, row.LastError)
	require.NotNil(t, row.CompletedAt)
	require.WithinDuration(t, completedAt, *row.CompletedAt, time.Millisecond)
}

func TestOperationClaimEligibility(t *testing.T) {
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
		{name: "executing active lease", status: "executing", lockedAt: &activeLock, want: false},
		{name: "executing expired lease", status: "executing", lockedAt: &expiredLock, want: true},
		{name: "completed", status: "completed", nextAttemptAt: now, want: false},
		{name: "failed", status: "failed", nextAttemptAt: now, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, operationClaimable(tt.status, tt.nextAttemptAt, tt.lockedAt, now, lease))
		})
	}
}

func testOperation(now time.Time) OperationRecord {
	return OperationRecord{
		ID:             uuid.NewString(),
		EventID:        uuid.NewString(),
		Subscriber:     "notification.build-completed",
		OperationType:  "notification.send",
		IdempotencyKey: uuid.NewString(),
		Payload:        []byte(`{"channel":"email"}`),
		NextAttemptAt:  now,
		CreatedAt:      now,
	}
}
