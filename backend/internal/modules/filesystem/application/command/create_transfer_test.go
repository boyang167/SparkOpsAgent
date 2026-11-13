package command

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
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
	require.Equal(t, cmd.ExpiresAt.UTC(), internal.ExpiresAt().UTC())
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
	registry := sharedevents.NewRegistry()
	if err := filesystemevent.Register(registry); err != nil {
		panic(err)
	}
	bus := messaging.NewReliableBus(registry, messagingpersistence.NewOutboxStore(db), nil, nil, nil)
	return NewCreateTransferHandler(
		persistence.NewRepository(db), postgres.NewTransactor(db), bus,
		auditService{store: audit.NewGormStore(db)},
	)
}

func transferFixture(ns *model.Namespace) CreateTransfer {
	return CreateTransfer{
		ActorID: uuid.New(), OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID,
		NamespaceID: ns.ID, ParentID: ns.RootNodeID, Name: "payload.bin",
		Protocol: model.TransferProtocolMultipart, Size: 60,
		SHA256: strings64("a"), Classification: model.TransferClassificationInternal,
		IdempotencyKey: "upload-1", ExpiresAt: time.Now().UTC().Add(time.Hour),
		ProviderID: uuid.New(), Authorization: Authorization{Allowed: true, TraceID: "trace-create"},
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
