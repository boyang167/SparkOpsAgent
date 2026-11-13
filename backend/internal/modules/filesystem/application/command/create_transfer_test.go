package command

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

func TestConcurrentReservationsDoNotOversell(t *testing.T) {
	db, ns, mainDSN, filesystemDSN, platformDSN := newConcurrentTransferTestDB(t)
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).
		Updates(map[string]any{"max_bytes": 100, "max_files": 10}).Error)

	start := make(chan struct{})
	ready := make(chan error, 2)
	errs := make(chan error, 2)
	for i := range 2 {
		i := i
		go func() {
			workerDB, err := openTransferWorkerDB(mainDSN, filesystemDSN, platformDSN)
			ready <- err
			if err != nil {
				errs <- err
				return
			}
			sqlDB, _ := workerDB.DB()
			defer sqlDB.Close()
			<-start
			cmd := transferFixture(ns)
			cmd.IdempotencyKey = "quota-" + string(rune('a'+i))
			_, err = newTransferHandler(workerDB).Handle(context.Background(), cmd)
			errs <- err
		}()
	}
	require.NoError(t, <-ready)
	require.NoError(t, <-ready)
	close(start)
	first, second := <-errs, <-errs
	require.True(t,
		(first == nil && errors.Is(second, domain.ErrQuotaExceeded)) ||
			(second == nil && errors.Is(first, domain.ErrQuotaExceeded)),
		"unexpected results: first=%v second=%v", first, second)

	var quota struct{ ReservedBytes, ReservedFiles int64 }
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Equal(t, int64(60), quota.ReservedBytes)
	require.Equal(t, int64(1), quota.ReservedFiles)
}

func TestCreateTransferConcurrentSameKeyHasOneSideEffectSet(t *testing.T) {
	db, ns, mainDSN, filesystemDSN, platformDSN := newConcurrentTransferTestDB(t)
	cmd := transferFixture(ns)
	start := make(chan struct{})
	ready := make(chan error, 2)
	results := make(chan TransferDescriptor, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			workerDB, err := openTransferWorkerDB(mainDSN, filesystemDSN, platformDSN)
			ready <- err
			if err != nil {
				errs <- err
				return
			}
			sqlDB, _ := workerDB.DB()
			defer sqlDB.Close()
			<-start
			result, handleErr := newTransferHandler(workerDB).Handle(context.Background(), cmd)
			results <- result
			errs <- handleErr
		}()
	}
	require.NoError(t, <-ready)
	require.NoError(t, <-ready)
	close(start)
	first, second := <-results, <-results
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Equal(t, first.SessionID, second.SessionID)
	encoded, err := json.Marshal(first)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "temporary")
	require.NotContains(t, string(encoded), "object_key")

	assertTransferSideEffectCounts(t, db, 1, 1, 1, 1)
	var reservation struct {
		SessionID uuid.UUID
		State     string
		ExpiresAt time.Time
	}
	require.NoError(t, db.Table("filesystem.quota_reservations").Take(&reservation).Error)
	require.Equal(t, first.SessionID, reservation.SessionID)
	require.Equal(t, "RESERVED", reservation.State)
	require.WithinDuration(t, cmd.ExpiresAt, reservation.ExpiresAt, time.Millisecond)
	internal, err := persistence.NewRepository(db).GetTransferByID(context.Background(), first.SessionID)
	require.NoError(t, err)
	require.NotEmpty(t, internal.TemporaryObjectKey())
	require.Equal(t, model.CanonicalTransferExpiry(cmd.ExpiresAt), internal.ExpiresAt().UTC())
}

func TestCreateTransferSameKeyDifferentParametersConflicts(t *testing.T) {
	db, ns := newTransferTestDB(t)
	handler := newTransferHandler(db)
	cmd := transferFixture(ns)
	_, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	cmd.Name = "different.bin"

	_, err = handler.Handle(context.Background(), cmd)

	require.ErrorIs(t, err, domain.ErrTransferIdempotencyConflict)
	assertTransferSideEffectCounts(t, db, 1, 1, 1, 2)
}

func TestCreateTransferCanonicalizesExpiryToPostgresMicroseconds(t *testing.T) {
	db, ns := newTransferTestDB(t)
	cmd := transferFixture(ns)
	cmd.ExpiresAt = time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond).Add(789 * time.Nanosecond)

	first, err := newTransferHandler(db).Handle(context.Background(), cmd)
	require.NoError(t, err)
	cmd.ExpiresAt = cmd.ExpiresAt.UTC().Truncate(time.Microsecond).Add(999 * time.Nanosecond)
	second, err := newTransferHandler(db).Handle(context.Background(), cmd)
	require.NoError(t, err)

	want := cmd.ExpiresAt.UTC().Truncate(time.Microsecond)
	require.Equal(t, first.SessionID, second.SessionID)
	require.Equal(t, want, first.ExpiresAt)
	require.Equal(t, want, second.ExpiresAt)
	var row struct {
		ExpiresAt   time.Time
		RequestHash string
	}
	require.NoError(t, db.Table("filesystem.transfer_sessions").Take(&row).Error)
	require.Equal(t, want, row.ExpiresAt.UTC())
	require.Len(t, row.RequestHash, 64)
	assertTransferSideEffectCounts(t, db, 1, 1, 1, 1)
}

func TestCreateTransferReplaysAuthoritativeExpiredSession(t *testing.T) {
	db, ns := newTransferTestDB(t)
	cmd := transferFixture(ns)
	cmd.ExpiresAt = time.Now().UTC().Add(30 * time.Millisecond).Truncate(time.Microsecond)
	first, err := newTransferHandler(db).Handle(context.Background(), cmd)
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	second, err := newTransferHandler(db).Handle(context.Background(), cmd)

	require.NoError(t, err)
	require.Equal(t, first, second)
	assertTransferSideEffectCounts(t, db, 1, 1, 1, 1)

	cmd.Size++
	_, err = newTransferHandler(db).Handle(context.Background(), cmd)
	require.ErrorIs(t, err, domain.ErrTransferIdempotencyConflict)
	assertTransferSideEffectCounts(t, db, 1, 1, 1, 2)
}

func TestCreateTransferChecksIdempotencyInsideTransaction(t *testing.T) {
	db, ns := newTransferTestDB(t)
	wrapped := &lookupTransactionRepository{FilesystemRepository: persistence.NewRepository(db)}

	_, err := newTransferHandlerWithRepository(db, wrapped).Handle(context.Background(), transferFixture(ns))

	require.NoError(t, err)
	require.True(t, wrapped.firstLookupInTransaction)
}

func TestCreateTransferPostgresReplaysAuthoritativeExpiredSessionWhenConfigured(t *testing.T) {
	db := openTransferPostgres(t)
	require.NoError(t, persistence.AutoMigrate(db))
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	require.NoError(t, audit.NewGormStore(db).AutoMigrate())
	repo := persistence.NewRepository(db)
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), ns))
	seedPutNamespaceRoot(t, repo, ns)
	cmd := transferFixture(ns)
	cmd.ExpiresAt = time.Now().UTC().Add(100 * time.Millisecond).Truncate(time.Microsecond).Add(789 * time.Nanosecond)

	first, err := newTransferHandler(db).Handle(context.Background(), cmd)
	require.NoError(t, err)
	time.Sleep(150 * time.Millisecond)
	cmd.ExpiresAt = cmd.ExpiresAt.UTC().Truncate(time.Microsecond).Add(999 * time.Nanosecond)
	second, err := newTransferHandler(db).Handle(context.Background(), cmd)

	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, cmd.ExpiresAt.UTC().Truncate(time.Microsecond), second.ExpiresAt)
	assertTransferSideEffectCounts(t, db, 1, 1, 1, 1)
}

func TestCreateTransferRejectsAuthorizationBoundToDifferentParent(t *testing.T) {
	db, ns := newTransferTestDB(t)
	repo := persistence.NewRepository(db)
	otherParent, err := model.NewDirectory(ns.ID, uuid.New(), "other", *ns.RootNodeID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), otherParent))
	cmd := transferFixture(ns)
	require.Equal(t, *ns.RootNodeID, cmd.Authorization.ResourceID)
	cmd.ParentID = &otherParent.ID

	_, err = newTransferHandler(db).Handle(context.Background(), cmd)

	require.ErrorIs(t, err, domain.ErrResourceDenied)
	var detail string
	require.NoError(t, db.Table("platform.audit_logs").Select("detail").Take(&detail).Error)
	require.Contains(t, detail, `"decision":"DENY"`)
	require.Contains(t, detail, `"reason":"authorization_scope_mismatch"`)
	assertTransferSideEffectCounts(t, db, 0, 0, 0, 1)
}

func TestCreateTransferRejectsMismatchedAuthorizationScope(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateTransfer)
	}{
		{"workspace", func(cmd *CreateTransfer) { cmd.Authorization.WorkspaceID = uuid.New() }},
		{"permission", func(cmd *CreateTransfer) { cmd.Authorization.Permission = model.ResourcePermissionRead }},
		{"unbound allowed", func(cmd *CreateTransfer) {
			cmd.Authorization = Authorization{Allowed: true, TraceID: cmd.TraceID}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, ns := newTransferTestDB(t)
			cmd := transferFixture(ns)
			test.mutate(&cmd)

			_, err := newTransferHandler(db).Handle(context.Background(), cmd)

			require.ErrorIs(t, err, domain.ErrUnauthorized)
			assertTransferSideEffectCounts(t, db, 0, 0, 0, 1)
		})
	}
}

func TestCreateTransferRollsBackSessionQuotaReservationAuditAndOutbox(t *testing.T) {
	db, ns := newTransferTestDB(t)
	handler := NewCreateTransferHandler(
		persistence.NewRepository(db), postgres.NewTransactor(db), failingBus{},
		auditService{store: audit.NewGormStore(db)},
	)

	_, err := handler.Handle(context.Background(), transferFixture(ns))

	require.Error(t, err)
	assertTransferSideEffectCounts(t, db, 0, 0, 0, 1)
	var quota struct{ ReservedBytes, ReservedFiles int64 }
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Zero(t, quota.ReservedBytes)
	require.Zero(t, quota.ReservedFiles)
}

func TestCreateTransferRejectsCrossWorkspaceParentAndAuditsStableReason(t *testing.T) {
	db, ns := newTransferTestDB(t)
	other := &model.Namespace{ID: uuid.New(), OrganizationID: ns.OrganizationID, WorkspaceID: uuid.New(), Status: "ACTIVE"}
	repo := persistence.NewRepository(db)
	require.NoError(t, repo.CreateNamespace(context.Background(), other))
	seedPutNamespaceRoot(t, repo, other)
	cmd := transferFixture(ns)
	cmd.ParentID = other.RootNodeID
	cmd.Authorization.ResourceID = *other.RootNodeID

	_, err := newTransferHandler(db).Handle(context.Background(), cmd)

	require.ErrorIs(t, err, domain.ErrParentNotFound)
	var detail string
	require.NoError(t, db.Table("platform.audit_logs").Select("detail").Take(&detail).Error)
	require.Contains(t, detail, `"reason":"parent_not_found"`)
	require.NotContains(t, detail, "temporary_object_key")
	require.NotContains(t, detail, "request_hash")
}

func TestCreateTransferAuthorizationAndValidationFailuresAreAudited(t *testing.T) {
	db, ns := newTransferTestDB(t)
	handler := newTransferHandler(db)
	denied := transferFixture(ns)
	denied.Authorization = Authorization{Allowed: false, Reason: "policy_deny", TraceID: "trace-deny"}
	_, err := handler.Handle(context.Background(), denied)
	require.ErrorIs(t, err, domain.ErrResourceDenied)

	invalid := transferFixture(ns)
	invalid.Size = maxTransferBytes + 1
	_, err = handler.Handle(context.Background(), invalid)
	require.ErrorIs(t, err, model.ErrInvalidTransferSize)

	var details []string
	require.NoError(t, db.Table("platform.audit_logs").Order("created_at").Pluck("detail", &details).Error)
	require.Len(t, details, 2)
	require.Contains(t, details[0], `"reason":"policy_deny"`)
	require.Contains(t, details[1], `"reason":"invalid_transfer_size"`)
}

func newTransferHandler(db *gorm.DB) *CreateTransferHandler {
	return newTransferHandlerWithRepository(db, persistence.NewRepository(db))
}

func newTransferHandlerWithRepository(db *gorm.DB, repo repository.FilesystemRepository) *CreateTransferHandler {
	registry := sharedevents.NewRegistry()
	if err := filesystemevent.Register(registry); err != nil {
		panic(err)
	}
	bus := messaging.NewReliableBus(registry, messagingpersistence.NewOutboxStore(db), nil, nil, nil)
	return NewCreateTransferHandler(
		repo, postgres.NewTransactor(db), bus,
		auditService{store: audit.NewGormStore(db)},
	)
}

type lookupTransactionRepository struct {
	repository.FilesystemRepository
	lookupChecked            bool
	firstLookupInTransaction bool
}

func (r *lookupTransactionRepository) GetTransferByIdempotency(ctx context.Context, workspaceID uuid.UUID, key string) (*model.TransferSession, error) {
	if !r.lookupChecked {
		r.lookupChecked = true
		_, err := postgres.RequireTx(ctx)
		r.firstLookupInTransaction = err == nil
	}
	return r.FilesystemRepository.GetTransferByIdempotency(ctx, workspaceID, key)
}

func transferFixture(ns *model.Namespace) CreateTransfer {
	return CreateTransfer{
		ActorID: uuid.New(), OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID,
		NamespaceID: ns.ID, ParentID: ns.RootNodeID, Name: "payload.bin",
		Protocol: model.TransferProtocolMultipart, Size: 60,
		SHA256: strings64("a"), Classification: model.TransferClassificationInternal,
		IdempotencyKey: "upload-1", ExpiresAt: time.Now().UTC().Add(time.Hour),
		ProviderID: uuid.New(), Authorization: Authorization{
			Allowed: true, TraceID: "trace-create", WorkspaceID: ns.WorkspaceID,
			ResourceID: *ns.RootNodeID, Permission: model.ResourcePermissionWrite,
		},
	}
}

func strings64(value string) string {
	out := ""
	for range 64 {
		out += value
	}
	return out
}

func newTransferTestDB(t *testing.T) (*gorm.DB, *model.Namespace) {
	t.Helper()
	db, repo, ns := newPutTestRepository(t)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	require.NoError(t, audit.NewGormStore(db).AutoMigrate())
	require.NotNil(t, repo)
	return db, ns
}

func newConcurrentTransferTestDB(t *testing.T) (*gorm.DB, *model.Namespace, string, string, string) {
	t.Helper()
	mainDSN := "file:" + uuid.NewString() + "?mode=memory&cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	filesystemDSN := filepath.Join(t.TempDir(), "filesystem.db")
	platformDSN := filepath.Join(t.TempDir(), "platform.db")
	db, err := openTransferWorkerDB(mainDSN, filesystemDSN, platformDSN)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, persistence.AutoMigrate(db))
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	require.NoError(t, audit.NewGormStore(db).AutoMigrate())
	repo := persistence.NewRepository(db)
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), ns))
	seedPutNamespaceRoot(t, repo, ns)
	return db, ns, mainDSN, filesystemDSN, platformDSN
}

func openTransferWorkerDB(mainDSN, filesystemDSN, platformDSN string) (*gorm.DB, error) {
	db, err := openPutWorkerDB(mainDSN, filesystemDSN)
	if err != nil {
		return nil, err
	}
	if err := db.Exec("ATTACH DATABASE ? AS platform", platformDSN).Error; err != nil {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func assertTransferSideEffectCounts(t *testing.T, db *gorm.DB, sessions, reservations, outbox, audits int64) {
	t.Helper()
	for table, want := range map[string]int64{
		"filesystem.transfer_sessions":  sessions,
		"filesystem.quota_reservations": reservations,
		"platform.event_outbox":         outbox,
		"platform.audit_logs":           audits,
	} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error, table)
		require.Equal(t, want, count, table)
	}
}

func openTransferPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN is not set")
	}
	admin, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	databaseName := "sparkops_g2_transfer_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE DATABASE " + databaseName).Error; err != nil {
		t.Skipf("cannot create isolated PostgreSQL test database: %v", err)
	}
	testDSN := dsn
	if strings.Contains(dsn, "://") {
		parsed, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		parsed.Path = "/" + databaseName
		testDSN = parsed.String()
	} else {
		databaseSetting := regexp.MustCompile(`(?i)\b(?:dbname|database)\s*=\s*(?:'[^']*'|\S+)`)
		if databaseSetting.MatchString(dsn) {
			testDSN = databaseSetting.ReplaceAllString(dsn, "dbname="+databaseName)
		} else {
			testDSN = dsn + " dbname=" + databaseName
		}
	}
	db, err := gorm.Open(gormpostgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, admin.Exec("DROP DATABASE "+databaseName).Error)
		adminDB, adminErr := admin.DB()
		require.NoError(t, adminErr)
		require.NoError(t, adminDB.Close())
	})
	return db
}
