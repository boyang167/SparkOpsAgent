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

func TestOperationInsertTreatsDuplicateIdempotencyKeyAsSuccess(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOperationStore(db)
	first := testOperation(time.Now().UTC())
	first.IdempotencyKey = "command-1:" + first.Subscriber + ":" + first.OperationType + ":target-1"
	insertTestOperation(t, db, store, first)
	duplicate := testOperation(time.Now().UTC())
	duplicate.IdempotencyKey = first.IdempotencyKey
	duplicate.Subscriber = first.Subscriber
	duplicate.OperationType = first.OperationType
	duplicate.Payload = append([]byte(nil), first.Payload...)
	afterDuplicate := testOperation(time.Now().UTC())

	err := postgres.NewTransactor(db).WithinTransaction(
		context.Background(),
		func(ctx context.Context) error {
			if err := store.Insert(ctx, duplicate); err != nil {
				return err
			}
			return store.Insert(ctx, afterDuplicate)
		},
	)

	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&OperationOutbox{}).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestOperationInsertRejectsIdempotencyKeyForDifferentIntent(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*OperationRecord)
	}{
		{
			name: "event identity",
			mutate: func(operation *OperationRecord) {
				operation.EventID = uuid.NewString()
			},
		},
		{
			name: "owner",
			mutate: func(operation *OperationRecord) {
				operation.Subscriber = "other.subscriber"
			},
		},
		{
			name: "operation type",
			mutate: func(operation *OperationRecord) {
				operation.OperationType = "other.operation"
			},
		},
		{
			name: "payload",
			mutate: func(operation *OperationRecord) {
				operation.Payload = []byte(`{"channel":"slack"}`)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openStoreSQLite(t)
			store := NewOperationStore(db)
			first := testOperation(time.Now().UTC())
			first.IdempotencyKey = first.EventID + ":" +
				first.Subscriber + ":" + first.OperationType + ":target-1"
			insertTestOperation(t, db, store, first)
			conflict := first
			conflict.ID = uuid.NewString()
			tt.mutate(&conflict)

			err := postgres.NewTransactor(db).WithinTransaction(
				context.Background(),
				func(ctx context.Context) error {
					return store.Insert(ctx, conflict)
				},
			)

			require.ErrorIs(t, err, ErrOperationIdempotencyConflict)
		})
	}
}

func TestOperationPayloadComparisonUsesExactJSONNumberSemantics(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		equal bool
	}{
		{
			name:  "object order and whitespace",
			left:  `{"count": 1, "nested": {"enabled": true}}`,
			right: `{"nested":{"enabled":true},"count":1}`,
			equal: true,
		},
		{
			name:  "equivalent number formats",
			left:  `{"values":[1,1.0,1e0]}`,
			right: `{"values":[1.00,1e+0,10e-1]}`,
			equal: true,
		},
		{
			name:  "adjacent integers above float precision",
			left:  `{"target":9007199254740992}`,
			right: `{"target":9007199254740993}`,
			equal: false,
		},
		{
			name:  "nested large integer mismatch",
			left:  `{"items":[{"id":123456789012345678901234567890}]}`,
			right: `{"items":[{"id":123456789012345678901234567891}]}`,
			equal: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(
				t,
				tt.equal,
				jsonPayloadEqual([]byte(tt.left), []byte(tt.right)),
			)
		})
	}
}

func TestOperationStatsIncludesPendingAndExecutingBacklog(t *testing.T) {
	db := openStoreSQLite(t)
	store := NewOperationStore(db)
	now := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	pending := testOperation(now.Add(-3 * time.Minute))
	pending.NextAttemptAt = now
	insertTestOperation(t, db, store, pending)
	executing := testOperation(now.Add(-time.Minute))
	executing.NextAttemptAt = now
	insertTestOperation(t, db, store, executing)
	setOperationLease(t, db, executing.ID, "worker-a")
	completed := testOperation(now.Add(-10 * time.Minute))
	completed.NextAttemptAt = now
	insertTestOperation(t, db, store, completed)
	setOperationLease(t, db, completed.ID, "worker-a")
	require.NoError(t, store.MarkCompleted(context.Background(), completed.ID, "worker-a", now))

	stats, err := store.Stats(context.Background(), now)

	require.NoError(t, err)
	require.Equal(t, int64(2), stats.Pending)
	require.Equal(t, 3*time.Minute, stats.OldestAge)
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
