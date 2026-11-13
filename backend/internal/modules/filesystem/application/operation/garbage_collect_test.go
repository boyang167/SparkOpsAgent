package operation

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/stretchr/testify/require"
)

func TestGCRefusesBlobWithLiveRevisionOrLegalHold(t *testing.T) {
	ctx := context.Background()
	gc := newGCTestEnv(t, DefaultGarbageCollectConfig())

	err := gc.collector.Delete(ctx, gc.blobWithLiveRevision())
	require.ErrorIs(t, err, domain.ErrBlobStillReferenced)

	err = gc.collector.Delete(ctx, gc.blobUnderLegalHold())
	require.ErrorIs(t, err, domain.ErrRetentionHold)
}

func TestGCRefusesBeforeGracePeriodExpires(t *testing.T) {
	ctx := context.Background()
	cfg := GarbageCollectConfig{GracePeriod: time.Hour}
	gc := newGCTestEnv(t, cfg)

	err := gc.collector.Delete(ctx, gc.readyBlobID)
	require.ErrorIs(t, err, domain.ErrBlobGCNotReady)
}

func TestGCSucceedsAndMarksDeletedWhenEligible(t *testing.T) {
	ctx := context.Background()
	cfg := GarbageCollectConfig{GracePeriod: time.Hour}
	gc := newGCTestEnv(t, cfg)
	gc.setTombstoned(gc.readyBlobID, time.Now().UTC().Add(-2*cfg.GracePeriod))

	require.NoError(t, gc.collector.Delete(ctx, gc.readyBlobID))

	blob, err := gc.repo.GetBlob(ctx, gc.readyBlobID)
	require.NoError(t, err)
	require.Equal(t, model.BlobStateDeleted, blob.State)

	replica, err := gc.repo.GetReplicaByProvider(ctx, gc.readyBlobID, gc.providerID)
	require.NoError(t, err)
	require.Equal(t, model.ReplicaStateDeleted, replica.State)
}

func TestGCMarksGCPendingOnPhysicalDeleteFailure(t *testing.T) {
	ctx := context.Background()
	cfg := GarbageCollectConfig{GracePeriod: time.Minute}
	gc := newGCTestEnv(t, cfg)
	gc.setTombstoned(gc.readyBlobID, time.Now().UTC().Add(-time.Hour))
	gc.provider.failDelete = errors.New("provider unavailable")

	err := gc.collector.Delete(ctx, gc.readyBlobID)
	require.Error(t, err)

	blob, getErr := gc.repo.GetBlob(ctx, gc.readyBlobID)
	require.NoError(t, getErr)
	require.Equal(t, model.BlobStateGCPending, blob.State)
}

type gcTestEnv struct {
	repo        *gcMemoryRepo
	provider    *gcDeleteProvider
	providerID  uuid.UUID
	readyBlobID uuid.UUID
	collector   *GarbageCollector
}

func newGCTestEnv(t *testing.T, cfg GarbageCollectConfig) *gcTestEnv {
	t.Helper()
	ctx := context.Background()
	repo := newGCMemoryRepo()
	providerID := uuid.New()
	provider := &gcDeleteProvider{}
	readyBlobID := uuid.New()
	replicaID := uuid.New()
	require.NoError(t, repo.InsertBlob(ctx, &model.Blob{
		ID: readyBlobID, DedupDomain: uuid.New(), SHA256: "a", Size: 1, RefCount: 0,
		State: model.BlobStateTombstoned,
	}))
	require.NoError(t, repo.InsertReplica(ctx, &model.StorageReplica{
		ID: replicaID, BlobID: readyBlobID, ProviderID: providerID,
		ObjectKey: "objects/test", State: model.ReplicaStateDraining,
	}))
	return &gcTestEnv{
		repo:        repo,
		provider:    provider,
		providerID:  providerID,
		readyBlobID: readyBlobID,
		collector:   NewGarbageCollector(repo, command.MapProviders{providerID: provider}, cfg),
	}
}

func (e *gcTestEnv) blobWithLiveRevision() uuid.UUID {
	id := uuid.New()
	_ = e.repo.InsertBlob(context.Background(), &model.Blob{
		ID: id, DedupDomain: uuid.New(), SHA256: "b", Size: 1, RefCount: 0, State: model.BlobStateTombstoned,
	})
	e.repo.liveRevisionBlobs[id] = struct{}{}
	return id
}

func (e *gcTestEnv) blobUnderLegalHold() uuid.UUID {
	id := uuid.New()
	_ = e.repo.InsertBlob(context.Background(), &model.Blob{
		ID: id, DedupDomain: uuid.New(), SHA256: "c", Size: 1, RefCount: 0,
		State: model.BlobStateTombstoned, LegalHold: true,
	})
	return id
}

func (e *gcTestEnv) setTombstoned(blobID uuid.UUID, at time.Time) {
	e.repo.tombstonedAt[blobID] = at
}

type gcMemoryRepo struct {
	blobs            map[uuid.UUID]*model.Blob
	replicas         map[uuid.UUID]map[uuid.UUID]*model.StorageReplica
	tombstonedAt     map[uuid.UUID]time.Time
	liveRevisionBlobs map[uuid.UUID]struct{}
	activeLinkBlobs   map[uuid.UUID]int64
}

func newGCMemoryRepo() *gcMemoryRepo {
	return &gcMemoryRepo{
		blobs: make(map[uuid.UUID]*model.Blob),
		replicas: make(map[uuid.UUID]map[uuid.UUID]*model.StorageReplica),
		tombstonedAt: make(map[uuid.UUID]time.Time),
		liveRevisionBlobs: make(map[uuid.UUID]struct{}),
		activeLinkBlobs: make(map[uuid.UUID]int64),
	}
}

func (r *gcMemoryRepo) InsertBlob(_ context.Context, blob *model.Blob) error {
	copy := *blob
	r.blobs[blob.ID] = &copy
	return nil
}

func (r *gcMemoryRepo) InsertReplica(_ context.Context, replica *model.StorageReplica) error {
	if r.replicas[replica.BlobID] == nil {
		r.replicas[replica.BlobID] = make(map[uuid.UUID]*model.StorageReplica)
	}
	copy := *replica
	r.replicas[replica.BlobID][replica.ProviderID] = &copy
	return nil
}

func (r *gcMemoryRepo) GetBlob(_ context.Context, blobID uuid.UUID) (*model.Blob, error) {
	blob, ok := r.blobs[blobID]
	if !ok {
		return nil, domain.ErrBlobConflict
	}
	copy := *blob
	return &copy, nil
}

func (r *gcMemoryRepo) GetReplicaByProvider(_ context.Context, blobID, providerID uuid.UUID) (*model.StorageReplica, error) {
	byProvider, ok := r.replicas[blobID]
	if !ok {
		return nil, domain.ErrReplicaConflict
	}
	replica, ok := byProvider[providerID]
	if !ok {
		return nil, domain.ErrReplicaConflict
	}
	copy := *replica
	return &copy, nil
}

func (r *gcMemoryRepo) ConfirmBlobGCEligible(_ context.Context, blobID uuid.UUID, grace time.Duration, now time.Time) (repository.BlobGCContext, error) {
	blob, ok := r.blobs[blobID]
	if !ok {
		return repository.BlobGCContext{}, domain.ErrBlobConflict
	}
	snapshot := repository.BlobGCContext{
		BlobID: blobID, State: blob.State, RefCount: blob.RefCount, LegalHold: blob.LegalHold,
		TombstonedAt: r.tombstonedAt[blobID],
	}
	if _, live := r.liveRevisionBlobs[blobID]; live {
		snapshot.LiveRevisionCount = 1
	}
	snapshot.ActiveLinkCount = r.activeLinkBlobs[blobID]
	if err := repository.ValidateBlobGCEligible(snapshot, grace, now); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (r *gcMemoryRepo) ListReplicasForGC(_ context.Context, blobID uuid.UUID) ([]model.StorageReplica, error) {
	byProvider := r.replicas[blobID]
	out := make([]model.StorageReplica, 0, len(byProvider))
	for _, replica := range byProvider {
		if replica.State == model.ReplicaStateDeleted {
			continue
		}
		out = append(out, *replica)
	}
	return out, nil
}

func (r *gcMemoryRepo) FinalizeBlobGC(_ context.Context, blobID uuid.UUID) error {
	blob := r.blobs[blobID]
	if blob == nil {
		return domain.ErrBlobConflict
	}
	blob.State = model.BlobStateDeleted
	for _, replica := range r.replicas[blobID] {
		replica.State = model.ReplicaStateDeleted
	}
	return nil
}

func (r *gcMemoryRepo) MarkBlobGCPending(_ context.Context, blobID uuid.UUID) error {
	blob := r.blobs[blobID]
	if blob == nil {
		return domain.ErrBlobConflict
	}
	blob.State = model.BlobStateGCPending
	return nil
}

type gcDeleteProvider struct {
	failDelete error
}

func (p *gcDeleteProvider) ID() string { return "gc-test" }

func (p *gcDeleteProvider) Capabilities() storage.Capabilities { return storage.Capabilities{} }

func (p *gcDeleteProvider) Delete(_ context.Context, objectKey string) error {
	if p.failDelete != nil {
		return p.failDelete
	}
	_ = objectKey
	return nil
}

func (p *gcDeleteProvider) Get(context.Context, storage.GetRequest) (io.ReadCloser, storage.ObjectInfo, error) {
	return nil, storage.ObjectInfo{}, errors.New("not implemented")
}
func (p *gcDeleteProvider) Put(context.Context, storage.PutRequest) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, errors.New("not implemented")
}
func (p *gcDeleteProvider) Stat(context.Context, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, errors.New("not implemented")
}
func (p *gcDeleteProvider) Copy(context.Context, string, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, errors.New("not implemented")
}
