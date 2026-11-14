package storage

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	platformstorage "github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

func TestRuntimeCatalogOpensProviderOnceAcrossConcurrentMisses(t *testing.T) {
	providerID := uuid.New()
	repo := &catalogRepositoryStub{
		provider: &model.StorageProvider{
			ID: providerID, ProviderType: model.ProviderTypeLocal,
			Endpoint: t.TempDir(), Status: "ACTIVE",
		},
		delay: 50 * time.Millisecond,
	}
	catalog, err := NewRuntimeCatalog(repo, RuntimeCatalogConfig{})
	require.NoError(t, err)

	const workers = 12
	start := make(chan struct{})
	results := make(chan platformstorage.Provider, workers)
	errs := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			provider, resolveErr := catalog.Provider(context.Background(), providerID)
			results <- provider
			errs <- resolveErr
		}()
	}
	close(start)
	group.Wait()
	close(results)
	close(errs)

	var first platformstorage.Provider
	for provider := range results {
		if first == nil {
			first = provider
		}
		require.Same(t, first, provider)
	}
	for resolveErr := range errs {
		require.NoError(t, resolveErr)
	}
	require.Equal(t, int32(1), repo.calls.Load())
	require.NoError(t, catalog.Close())
}

func TestRuntimeCatalogImplementsCloserAndClosesProviders(t *testing.T) {
	fallback := &closableProvider{}
	cached := &closableProvider{}
	catalog := &RuntimeCatalog{
		fallback: fallback,
		cache:    map[uuid.UUID]platformstorage.Provider{uuid.New(): cached},
	}

	closer, ok := any(catalog).(io.Closer)
	require.True(t, ok, "runtime catalog must release cached provider resources")
	require.NoError(t, closer.Close())
	require.NoError(t, closer.Close())
	require.Equal(t, int32(1), fallback.closed.Load())
	require.Equal(t, int32(1), cached.closed.Load())
}

type catalogRepositoryStub struct {
	repository.FilesystemRepository
	provider *model.StorageProvider
	delay    time.Duration
	calls    atomic.Int32
}

func (r *catalogRepositoryStub) GetStorageProvider(context.Context, uuid.UUID) (*model.StorageProvider, error) {
	r.calls.Add(1)
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	return r.provider, nil
}

type closableProvider struct {
	closed atomic.Int32
}

func (*closableProvider) ID() string { return "closable" }

func (*closableProvider) Capabilities() platformstorage.Capabilities {
	return platformstorage.Capabilities{}
}

func (*closableProvider) Put(context.Context, platformstorage.PutRequest) (platformstorage.ObjectInfo, error) {
	return platformstorage.ObjectInfo{}, platformstorage.ErrNotFound
}

func (*closableProvider) Get(context.Context, platformstorage.GetRequest) (io.ReadCloser, platformstorage.ObjectInfo, error) {
	return nil, platformstorage.ObjectInfo{}, platformstorage.ErrNotFound
}

func (*closableProvider) Stat(context.Context, string) (platformstorage.ObjectInfo, error) {
	return platformstorage.ObjectInfo{}, platformstorage.ErrNotFound
}

func (*closableProvider) Delete(context.Context, string) error { return nil }

func (*closableProvider) Copy(context.Context, string, string) (platformstorage.ObjectInfo, error) {
	return platformstorage.ObjectInfo{}, platformstorage.ErrNotFound
}

func (p *closableProvider) Close() error {
	p.closed.Add(1)
	return nil
}
