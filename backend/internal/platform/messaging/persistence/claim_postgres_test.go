//go:build integration

package persistence

import (
	"context"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOutboxConcurrentClaimsAreDisjointPostgres(t *testing.T) {
	db := openStorePostgres(t)
	store := NewOutboxStore(db)
	insertOutboxRecords(t, db, store, time.Now().UTC(), 4)
	now := time.Now().UTC()

	start := make(chan struct{})
	results := make(chan []OutboxRecord, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, owner := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			<-start
			records, err := store.Claim(context.Background(), owner, 2, now, time.Minute)
			results <- records
			errs <- err
		}(owner)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	var claims [][]OutboxRecord
	for records := range results {
		require.Len(t, records, 2)
		claims = append(claims, records)
	}
	require.Len(t, claims, 2)
	first := recordIDs(claims[0])
	for _, record := range claims[1] {
		require.NotContains(t, first, record.ID)
	}
}

func TestOutboxClaimRecoversOnlyExpiredPublishingLeasePostgres(t *testing.T) {
	db := openStorePostgres(t)
	store := NewOutboxStore(db)
	ids := insertOutboxRecords(t, db, store, time.Now().UTC(), 1)
	now := time.Now().UTC()

	first, err := store.Claim(context.Background(), "worker-a", 1, now, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, ids[0], first[0].ID)
	require.Equal(t, 1, first[0].Attempts)

	active, err := store.Claim(context.Background(), "worker-b", 10, now.Add(30*time.Second), time.Minute)
	require.NoError(t, err)
	require.NotContains(t, recordIDs(active), ids[0])

	expired, err := store.Claim(context.Background(), "worker-b", 10, now.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.Contains(t, recordIDs(expired), ids[0])
}

func TestOutboxOldOwnerCannotOverwriteRecoveredLeasePostgres(t *testing.T) {
	db := openStorePostgres(t)
	store := NewOutboxStore(db)
	ids := insertOutboxRecords(t, db, store, time.Now().UTC(), 1)
	now := time.Now().UTC()

	first, err := store.Claim(context.Background(), "worker-a", 1, now, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	recovered, err := store.Claim(context.Background(), "worker-b", 1, now.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.Len(t, recovered, 1)

	require.ErrorIs(t,
		store.MarkPublished(context.Background(), ids[0], "worker-a", now.Add(time.Minute)),
		ErrLeaseLost,
	)
	require.ErrorIs(t,
		store.Reschedule(context.Background(), ids[0], "worker-a", "stale", now.Add(2*time.Minute)),
		ErrLeaseLost,
	)
	require.NoError(t,
		store.MarkPublished(context.Background(), ids[0], "worker-b", now.Add(time.Minute)),
	)
}

func TestOperationConcurrentClaimsAreDisjointPostgres(t *testing.T) {
	db := openStorePostgres(t)
	store := NewOperationStore(db)
	insertOperationRecords(t, db, store, time.Now().UTC(), 4)
	now := time.Now().UTC()

	start := make(chan struct{})
	results := make(chan []OperationRecord, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, owner := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			<-start
			records, err := store.Claim(context.Background(), owner, 2, now, time.Minute)
			results <- records
			errs <- err
		}(owner)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	var claims [][]OperationRecord
	for records := range results {
		require.Len(t, records, 2)
		claims = append(claims, records)
	}
	require.Len(t, claims, 2)
	first := operationRecordIDs(claims[0])
	for _, record := range claims[1] {
		require.NotContains(t, first, record.ID)
	}
}

func TestOperationClaimRecoversOnlyExpiredExecutingLeasePostgres(t *testing.T) {
	db := openStorePostgres(t)
	store := NewOperationStore(db)
	operation := testOperation(time.Now().UTC())
	transactor := postgres.NewTransactor(db)
	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return store.Insert(ctx, operation)
	}))
	now := time.Now().UTC()
	t.Cleanup(func() {
		db.Exec("DELETE FROM platform.operation_outbox WHERE id = ?", operation.ID)
	})

	first, err := store.Claim(context.Background(), "worker-a", 1, now, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, operation.ID, first[0].ID)
	require.Equal(t, 1, first[0].Attempts)

	active, err := store.Claim(context.Background(), "worker-b", 10, now.Add(30*time.Second), time.Minute)
	require.NoError(t, err)
	require.NotContains(t, operationRecordIDs(active), operation.ID)

	expired, err := store.Claim(context.Background(), "worker-b", 10, now.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.Contains(t, operationRecordIDs(expired), operation.ID)
}

func TestOperationOldOwnerCannotOverwriteRecoveredLeasePostgres(t *testing.T) {
	db := openStorePostgres(t)
	store := NewOperationStore(db)
	operations := insertOperationRecords(t, db, store, time.Now().UTC(), 1)
	now := time.Now().UTC()

	first, err := store.Claim(context.Background(), "worker-a", 1, now, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	recovered, err := store.Claim(context.Background(), "worker-b", 1, now.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.Len(t, recovered, 1)

	require.ErrorIs(t,
		store.MarkCompleted(context.Background(), operations[0].ID, "worker-a", now.Add(time.Minute)),
		ErrLeaseLost,
	)
	require.ErrorIs(t,
		store.MarkFailed(context.Background(), operations[0].ID, "worker-a", "stale", now.Add(time.Minute)),
		ErrLeaseLost,
	)
	require.ErrorIs(t,
		store.Reschedule(context.Background(), operations[0].ID, "worker-a", "stale", now.Add(2*time.Minute)),
		ErrLeaseLost,
	)
	require.NoError(t,
		store.MarkCompleted(context.Background(), operations[0].ID, "worker-b", now.Add(time.Minute)),
	)
}

func TestOperationIdempotencyIntentComparisonPostgres(t *testing.T) {
	db := openStorePostgres(t)
	store := NewOperationStore(db)
	first := testOperation(time.Now().UTC())
	first.IdempotencyKey = "command-1:" + first.Subscriber + ":" + first.OperationType + ":target-1"
	insertTestOperation(t, db, store, first)

	same := first
	same.ID = uuid.NewString()
	same.EventID = uuid.NewString()
	require.NoError(t, postgres.NewTransactor(db).WithinTransaction(
		context.Background(),
		func(ctx context.Context) error {
			return store.Insert(ctx, same)
		},
	))

	conflict := first
	conflict.ID = uuid.NewString()
	conflict.Payload = []byte(`{"channel":"slack"}`)
	err := postgres.NewTransactor(db).WithinTransaction(
		context.Background(),
		func(ctx context.Context) error {
			return store.Insert(ctx, conflict)
		},
	)
	require.ErrorIs(t, err, ErrOperationIdempotencyConflict)

	var count int64
	require.NoError(t, db.Model(&OperationOutbox{}).
		Where("idempotency_key = ?", first.IdempotencyKey).
		Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestOperationEventIDRequiresUUIDPostgres(t *testing.T) {
	db := openStorePostgres(t)
	store := NewOperationStore(db)
	operation := testOperation(time.Now().UTC())
	operation.EventID = "stable-command-name"

	err := postgres.NewTransactor(db).WithinTransaction(
		context.Background(),
		func(ctx context.Context) error {
			return store.Insert(ctx, operation)
		},
	)

	require.Error(t, err)
	require.ErrorContains(t, err, "invalid input syntax for type uuid")
}

func openStorePostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN is not set")
	}
	admin, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	databaseName := "sparkops_task3_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE DATABASE " + databaseName).Error; err != nil {
		t.Skipf("cannot create isolated PostgreSQL test database: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(dsnWithDatabase(t, dsn, databaseName)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, admin.Exec("DROP DATABASE "+databaseName).Error)
		adminSQL, adminErr := admin.DB()
		require.NoError(t, adminErr)
		require.NoError(t, adminSQL.Close())
	})
	require.NoError(t, AutoMigrate(db))
	return db
}

func dsnWithDatabase(t *testing.T, dsn, databaseName string) string {
	t.Helper()
	if strings.Contains(dsn, "://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		parsed.Path = "/" + databaseName
		return parsed.String()
	}
	databaseSetting := regexp.MustCompile(`(?i)\b(?:dbname|database)\s*=\s*(?:'[^']*'|\S+)`)
	if databaseSetting.MatchString(dsn) {
		return databaseSetting.ReplaceAllString(dsn, "dbname="+databaseName)
	}
	return dsn + " dbname=" + databaseName
}

func insertOutboxRecords(t *testing.T, db *gorm.DB, store OutboxStore, now time.Time, count int) []string {
	t.Helper()
	ids := make([]string, 0, count)
	transactor := postgres.NewTransactor(db)
	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		for range count {
			envelope := testEnvelope(now)
			ids = append(ids, envelope.ID)
			if err := store.Insert(ctx, envelope); err != nil {
				return err
			}
			tx, err := postgres.RequireTx(ctx)
			if err != nil {
				return err
			}
			if err := tx.Model(&EventOutbox{}).Where("id = ?", envelope.ID).
				Update("next_attempt_at", now).Error; err != nil {
				return err
			}
		}
		return nil
	}))
	t.Cleanup(func() {
		db.Exec("DELETE FROM platform.event_outbox WHERE id IN ?", ids)
	})
	return ids
}

func insertOperationRecords(
	t *testing.T,
	db *gorm.DB,
	store OperationStore,
	now time.Time,
	count int,
) []OperationRecord {
	t.Helper()
	operations := make([]OperationRecord, 0, count)
	transactor := postgres.NewTransactor(db)
	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		for range count {
			operation := testOperation(now)
			operations = append(operations, operation)
			if err := store.Insert(ctx, operation); err != nil {
				return err
			}
		}
		return nil
	}))
	t.Cleanup(func() {
		ids := make([]string, 0, len(operations))
		for _, operation := range operations {
			ids = append(ids, operation.ID)
		}
		db.Exec("DELETE FROM platform.operation_outbox WHERE id IN ?", ids)
	})
	return operations
}

func recordIDs(records []OutboxRecord) map[string]struct{} {
	ids := make(map[string]struct{}, len(records))
	for _, record := range records {
		ids[record.ID] = struct{}{}
	}
	return ids
}

func operationRecordIDs(records []OperationRecord) map[string]struct{} {
	ids := make(map[string]struct{}, len(records))
	for _, record := range records {
		ids[record.ID] = struct{}{}
	}
	return ids
}
