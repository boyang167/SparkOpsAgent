package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
	const owner = "worker-a"
	operation := testOperation(time.Now().UTC())
	insertTestOperation(t, db, store, operation)

	setOperationLease(t, db, operation.ID, owner)
	next := operation.NextAttemptAt.Add(time.Minute)
	require.NoError(t, store.Reschedule(context.Background(), operation.ID, owner, "temporary", next))
	var row OperationOutbox
	require.NoError(t, db.First(&row, "id = ?", operation.ID).Error)
	require.Equal(t, "pending", row.Status)
	require.Equal(t, "temporary", row.LastError)
	require.WithinDuration(t, next, row.NextAttemptAt, time.Millisecond)

	setOperationLease(t, db, operation.ID, owner)
	failedAt := next.Add(time.Minute)
	require.NoError(t, store.MarkFailed(context.Background(), operation.ID, owner, "permanent", failedAt))
	require.NoError(t, db.First(&row, "id = ?", operation.ID).Error)
	require.Equal(t, "failed", row.Status)
	require.Equal(t, "permanent", row.LastError)
	require.NotNil(t, row.CompletedAt)
	require.WithinDuration(t, failedAt, *row.CompletedAt, time.Millisecond)

	completedOperation := testOperation(time.Now().UTC())
	insertTestOperation(t, db, store, completedOperation)
	setOperationLease(t, db, completedOperation.ID, owner)
	completedAt := failedAt.Add(time.Minute)
	require.NoError(t, store.MarkCompleted(context.Background(), completedOperation.ID, owner, completedAt))
	row = OperationOutbox{}
	require.NoError(t, db.First(&row, "id = ?", completedOperation.ID).Error)
	require.Equal(t, "completed", row.Status)
	require.Empty(t, row.LastError)
	require.NotNil(t, row.CompletedAt)
	require.WithinDuration(t, completedAt, *row.CompletedAt, time.Millisecond)
}

func TestOperationUpdatesRejectLostLease(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOperationStore(db)
	operation := testOperation(time.Now().UTC())
	insertTestOperation(t, db, store, operation)
	setOperationLease(t, db, operation.ID, "new-owner")

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "complete",
			call: func() error {
				return store.MarkCompleted(context.Background(), operation.ID, "old-owner", time.Now().UTC())
			},
		},
		{
			name: "fail",
			call: func() error {
				return store.MarkFailed(context.Background(), operation.ID, "old-owner", "permanent", time.Now().UTC())
			},
		},
		{
			name: "reschedule",
			call: func() error {
				return store.Reschedule(context.Background(), operation.ID, "old-owner", "temporary", time.Now().UTC())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, tt.call(), ErrLeaseLost)
		})
	}
	var row OperationOutbox
	require.NoError(t, db.First(&row, "id = ?", operation.ID).Error)
	require.Equal(t, "executing", row.Status)
	require.Equal(t, "new-owner", row.LockedBy)
}

func TestOperationUpdatesRejectMissingOrWrongState(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOperationStore(db)
	missingID := uuid.NewString()

	require.ErrorIs(t,
		store.MarkCompleted(context.Background(), missingID, "worker-a", time.Now().UTC()),
		ErrLeaseLost,
	)
	require.ErrorIs(t,
		store.MarkFailed(context.Background(), missingID, "worker-a", "permanent", time.Now().UTC()),
		ErrLeaseLost,
	)
	require.ErrorIs(t,
		store.Reschedule(context.Background(), missingID, "worker-a", "temporary", time.Now().UTC()),
		ErrLeaseLost,
	)

	operation := testOperation(time.Now().UTC())
	insertTestOperation(t, db, store, operation)
	require.ErrorIs(t,
		store.MarkCompleted(context.Background(), operation.ID, "worker-a", time.Now().UTC()),
		ErrLeaseLost,
	)
}

func TestOperationTerminalStatesCannotBeRescheduled(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOperationStore(db)
	const owner = "worker-a"

	tests := []struct {
		name     string
		finalize func(string) error
	}{
		{
			name: "completed",
			finalize: func(id string) error {
				return store.MarkCompleted(context.Background(), id, owner, time.Now().UTC())
			},
		},
		{
			name: "failed",
			finalize: func(id string) error {
				return store.MarkFailed(context.Background(), id, owner, "permanent", time.Now().UTC())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation := testOperation(time.Now().UTC())
			insertTestOperation(t, db, store, operation)
			setOperationLease(t, db, operation.ID, owner)
			require.NoError(t, tt.finalize(operation.ID))
			require.ErrorIs(t,
				store.Reschedule(context.Background(), operation.ID, owner, "temporary", time.Now().UTC()),
				ErrLeaseLost,
			)
		})
	}
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

func insertTestOperation(t *testing.T, db *gorm.DB, store OperationStore, operation OperationRecord) {
	t.Helper()
	transactor := postgres.NewTransactor(db)
	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return store.Insert(ctx, operation)
	}))
}

func setOperationLease(t *testing.T, db *gorm.DB, id, owner string) {
	t.Helper()
	require.NoError(t, db.Model(&OperationOutbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":    "executing",
		"locked_by": owner,
		"locked_at": time.Now().UTC(),
	}).Error)
}
