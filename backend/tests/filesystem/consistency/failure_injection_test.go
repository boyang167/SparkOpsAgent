//go:build integration

package consistency_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	s3storage "github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/s3"
	testenv "github.com/sparkops/sparkops-agent/backend/tests/filesystem/integration"
)

func TestFailureInjectionRecoveryInvariants(t *testing.T) {
	testenv.Require(t)
	t.Run("object write followed by database failure leaves no metadata", testObjectWriteThenDBFailure)
	t.Run("paused outbox retains pending event", testPausedOutbox)
	t.Run("worker crash before acknowledgement replays idempotently", testWorkerCrashReplay)
	t.Run("multipart completion rejects a missing part", testMissingMultipartPart)
	t.Run("interrupted migration preserves checkpoint", testInterruptedMigration)
	t.Run("lost lease prevents stale reconciler commit", testLeaseLoss)
}

func testObjectWriteThenDBFailure(t *testing.T) {
	db, repo, _ := newFilesystemDB(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	content := []byte("orphan candidate")
	objectKey := "objects/" + uuid.NewString()
	_, err = provider.Put(context.Background(), storage.PutRequest{
		ObjectKey: objectKey, Body: bytes.NewReader(content),
		Size: int64(len(content)), SHA256: storage.SHA256(content),
	})
	require.NoError(t, err)

	blobID := uuid.New()
	err = postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NoError(t, repo.InsertBlob(ctx, &model.Blob{
			ID: blobID, DedupDomain: uuid.New(), SHA256: storage.SHA256(content),
			Size: int64(len(content)), RefCount: 1, State: model.BlobStateAvailable,
		}))
		return errors.New("injected database failure")
	})
	require.ErrorContains(t, err, "injected database failure")
	_, err = repo.GetBlob(context.Background(), blobID)
	require.ErrorIs(t, err, domain.ErrBlobConflict)

	require.NoError(t, provider.Delete(context.Background(), objectKey))
	_, err = provider.Stat(context.Background(), objectKey)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func testPausedOutbox(t *testing.T) {
	db, _, _ := newFilesystemDB(t)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	now := time.Now().UTC().Add(-10 * time.Minute)
	require.NoError(t, db.Create(&messagingpersistence.EventOutbox{
		ID: uuid.NewString(), EventType: "filesystem.transfer.uploaded.v1", EventVersion: 1,
		Payload: []byte(`{}`), OccurredAt: now, Status: "pending",
		NextAttemptAt: now, CreatedAt: now,
	}).Error)

	stats, err := messagingpersistence.NewOutboxStore(db).Stats(context.Background(), time.Now().UTC())
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Pending)
	require.GreaterOrEqual(t, stats.OldestAge, 10*time.Minute)
}

func testWorkerCrashReplay(t *testing.T) {
	db, repo, ns := newFilesystemDB(t)
	handler := command.NewCreateTransferHandler(repo, postgres.NewTransactor(db), nil, nil, nil)
	cmd := command.CreateTransfer{
		ActorID: uuid.New(), OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID,
		NamespaceID: ns.ID, ParentID: ns.RootNodeID, Name: "payload.bin",
		Protocol: model.TransferProtocolMultipart, Size: 60, SHA256: strings.Repeat("a", 64),
		Classification: model.TransferClassificationInternal, IdempotencyKey: "worker-crash-replay",
		ExpiresAt: time.Now().UTC().Add(time.Hour), ProviderID: uuid.New(),
		Authorization: command.Authorization{
			Allowed: true, WorkspaceID: ns.WorkspaceID, ResourceID: *ns.RootNodeID,
			Permission: model.ResourcePermissionWrite,
		},
	}

	first, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	second, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, first.SessionID, second.SessionID)

	var sessions, reservations int64
	require.NoError(t, db.Table("filesystem.transfer_sessions").Count(&sessions).Error)
	require.NoError(t, db.Table("filesystem.quota_reservations").Count(&reservations).Error)
	require.EqualValues(t, 1, sessions)
	require.EqualValues(t, 1, reservations)
}

func testMissingMultipartPart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Has("uploads"):
			_, _ = w.Write([]byte(`<CreateMultipartUploadResult><Bucket>test</Bucket><Key>objects/test</Key><UploadId>upload-1</UploadId></CreateMultipartUploadResult>`))
		case r.Method == http.MethodPost && r.URL.Query().Get("uploadId") == "upload-1":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`<Error><Code>InvalidPart</Code><Message>part 2 is missing</Message></Error>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	provider, err := s3storage.New(s3storage.Config{
		Endpoint: server.URL, Region: "us-east-1", Bucket: "test", PathStyle: true,
		AccessKey: "test", SecretKey: "test",
	})
	require.NoError(t, err)
	upload, err := provider.CreateMultipart(context.Background(), "objects/test", 10)
	require.NoError(t, err)

	_, err = provider.CompleteMultipart(context.Background(), upload.UploadID, []storage.CompletedPart{
		{PartNumber: 1, ETag: "etag-1"},
	})
	require.ErrorIs(t, err, storage.ErrInvalidPart)
}

func testInterruptedMigration(t *testing.T) {
	_, repo, _ := newFilesystemDB(t)
	blobID, targetProviderID := uuid.New(), uuid.New()
	require.NoError(t, repo.InsertBlob(context.Background(), &model.Blob{
		ID: blobID, DedupDomain: uuid.New(), SHA256: strings.Repeat("b", 64),
		Size: 1024, RefCount: 1, State: model.BlobStateAvailable,
	}))
	checkpoint := repository.ReplicaMigrationCheckpoint{
		BlobID: blobID, TargetProviderID: targetProviderID, TargetReplicaID: uuid.New(),
		SourceProviderID: uuid.New(), BytesCopied: 512, TargetObjectKey: "replica/partial", PolicyVersion: 7,
	}
	require.NoError(t, repo.SaveReplicaMigrationCheckpoint(context.Background(), checkpoint))

	copied, err := repo.GetReplicaMigrationCheckpoint(context.Background(), blobID, targetProviderID)
	require.NoError(t, err)
	require.EqualValues(t, 512, copied)
	require.NoError(t, repo.SaveReplicaMigrationCheckpoint(context.Background(), repository.ReplicaMigrationCheckpoint{
		BlobID: blobID, TargetProviderID: targetProviderID, TargetReplicaID: checkpoint.TargetReplicaID,
		SourceProviderID: checkpoint.SourceProviderID, BytesCopied: 1024,
		TargetObjectKey: checkpoint.TargetObjectKey, PolicyVersion: checkpoint.PolicyVersion,
	}))
	copied, err = repo.GetReplicaMigrationCheckpoint(context.Background(), blobID, targetProviderID)
	require.NoError(t, err)
	require.EqualValues(t, 1024, copied)
}

func testLeaseLoss(t *testing.T) {
	db, _, _ := newFilesystemDB(t)
	lease, err := service.NewDBLease(db)
	require.NoError(t, err)
	now := time.Now().UTC()
	acquired, err := lease.TryAcquireOrRenew(context.Background(), "filesystem-reconciler", "worker-a", time.Second, now)
	require.NoError(t, err)
	require.True(t, acquired)
	acquired, err = lease.TryAcquireOrRenew(context.Background(), "filesystem-reconciler", "worker-b", time.Second, now.Add(2*time.Second))
	require.NoError(t, err)
	require.True(t, acquired)

	err = lease.RecordPass(context.Background(), "filesystem-reconciler", "worker-a", service.ReconcileSummary{}, now.Add(2*time.Second))
	require.ErrorContains(t, err, "lease lost")
}

func newFilesystemDB(t *testing.T) (*gorm.DB, repository.FilesystemRepository, *model.Namespace) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	namespace := &model.Namespace{
		ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	root, err := model.NewRootDirectory(namespace.ID, uuid.New(), ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), root))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), namespace.WorkspaceID, root.ID))
	namespace.RootNodeID = &root.ID
	return db, repo, namespace
}
