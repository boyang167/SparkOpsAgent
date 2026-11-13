package operation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const replicationBlobSize = 1024

func TestReplicationResumesWithoutSwitchingBeforeVerification(t *testing.T) {
	ctx := context.Background()
	env := newReplicationTestEnv(t)

	payload := ReplicateBlobPayload{
		BlobID:           env.blobID.String(),
		SourceProviderID: env.sourceProviderID.String(),
		TargetProviderID: env.targetProviderID.String(),
		PolicyVersion:    1,
	}
	op := env.handler(failAfterBytes(512))

	err := op.Execute(ctx, payload)
	require.Error(t, err)
	env.assertSourceStillPreferred(t)
	env.assertCheckpoint(t, 512)

	require.NoError(t, op.Execute(ctx, payload))
	env.assertTargetAvailableAndPreferred(t)
	env.assertSourceDraining(t)
}

type replicationTestEnv struct {
	t                *testing.T
	db               *gorm.DB
	repo             repository.FilesystemRepository
	blobID           uuid.UUID
	sourceProviderID uuid.UUID
	targetProviderID uuid.UUID
	sourceProvider   *local.Provider
	targetProvider   *local.Provider
	trackedTarget    *trackedProvider
}

func (e *replicationTestEnv) handler(failHook failAfterHook) *ReplicateBlobHandler {
	e.trackedTarget.failHook = failHook
	catalog := command.MapProviders{
		e.sourceProviderID: e.sourceProvider,
		e.targetProviderID: e.trackedTarget,
	}
	transactor := postgres.NewTransactor(e.db)
	promoter := command.NewPromoteReplicaHandler(e.repo, transactor)
	return NewReplicateBlobHandler(e.repo, catalog, promoter, DefaultReplicationConfig())
}

func (e *replicationTestEnv) assertSourceStillPreferred(t *testing.T) {
	t.Helper()
	replica, err := e.repo.GetPreferredReplica(context.Background(), e.blobID)
	require.NoError(t, err)
	require.Equal(t, e.sourceProviderID, replica.ProviderID)
}

func (e *replicationTestEnv) assertCheckpoint(t *testing.T, bytesCopied int64) {
	t.Helper()
	checkpoint, err := e.repo.GetReplicaMigrationCheckpoint(context.Background(), e.blobID, e.targetProviderID)
	require.NoError(t, err)
	require.Equal(t, bytesCopied, checkpoint)
}

func (e *replicationTestEnv) assertTargetAvailableAndPreferred(t *testing.T) {
	t.Helper()
	target, err := e.repo.GetReplicaByProvider(context.Background(), e.blobID, e.targetProviderID)
	require.NoError(t, err)
	require.Equal(t, model.ReplicaStateAvailable, target.State)
	require.NotNil(t, target.VerifiedAt)

	replica, err := e.repo.GetPreferredReplica(context.Background(), e.blobID)
	require.NoError(t, err)
	require.Equal(t, e.targetProviderID, replica.ProviderID)
}

func (e *replicationTestEnv) assertSourceDraining(t *testing.T) {
	t.Helper()
	source, err := e.repo.GetReplicaByProvider(context.Background(), e.blobID, e.sourceProviderID)
	require.NoError(t, err)
	require.Equal(t, model.ReplicaStateDraining, source.State)
	require.NotNil(t, source.DeleteAfter)
}

func newReplicationTestEnv(t *testing.T) *replicationTestEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)

	sourceProviderID := uuid.New()
	targetProviderID := uuid.New()

	sourceProvider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sourceProvider.Close() })
	targetProvider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = targetProvider.Close() })
	tracked := &trackedProvider{Provider: targetProvider}
	data := make([]byte, replicationBlobSize)
	for i := range data {
		data[i] = byte(i % 251)
	}
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	objectKey := "objects/" + uuid.NewString()
	_, err = sourceProvider.Put(context.Background(), storage.PutRequest{
		ObjectKey: objectKey,
		Body:      bytes.NewReader(data),
		Size:      int64(len(data)),
		SHA256:    checksum,
	})
	require.NoError(t, err)

	blobID := uuid.New()
	verifiedAt := time.Now().UTC()
	require.NoError(t, repo.InsertBlob(context.Background(), &model.Blob{
		ID: blobID, DedupDomain: uuid.New(), SHA256: checksum, Size: int64(len(data)),
		RefCount: 1, State: model.BlobStateAvailable,
	}))
	require.NoError(t, repo.InsertReplica(context.Background(), &model.StorageReplica{
		ID: uuid.New(), BlobID: blobID, ProviderID: sourceProviderID,
		ObjectKey: objectKey, Checksum: checksum, State: model.ReplicaStateAvailable, VerifiedAt: &verifiedAt,
	}))
	require.NoError(t, repo.SetPreferredProvider(context.Background(), blobID, sourceProviderID))

	return &replicationTestEnv{
		t: t, db: db, repo: repo, blobID: blobID,
		sourceProviderID: sourceProviderID, targetProviderID: targetProviderID,
		sourceProvider: sourceProvider, targetProvider: targetProvider, trackedTarget: tracked,
	}
}

type failAfterHook func(chunkBytes int64, totalCopied int64) error

func failAfterBytes(limit int64) failAfterHook {
	failed := false
	return func(_ int64, totalCopied int64) error {
		if failed {
			return nil
		}
		if totalCopied > limit {
			failed = true
			return errors.New("injected replication failure")
		}
		return nil
	}
}

type trackedProvider struct {
	storage.Provider
	failHook failAfterHook
	copied   int64
}

func (p *trackedProvider) Put(ctx context.Context, req storage.PutRequest) (storage.ObjectInfo, error) {
	if req.Body != nil && p.failHook != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return storage.ObjectInfo{}, err
		}
		p.copied = int64(len(data))
		if err := p.failHook(int64(len(data)), p.copied); err != nil {
			return storage.ObjectInfo{}, err
		}
		req.Body = bytes.NewReader(data)
		req.Size = int64(len(data))
		if req.SHA256 == "" {
			sum := sha256.Sum256(data)
			req.SHA256 = hex.EncodeToString(sum[:])
		}
	}
	return p.Provider.Put(ctx, req)
}
