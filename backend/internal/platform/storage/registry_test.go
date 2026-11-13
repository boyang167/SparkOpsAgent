package storage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/wire"
)

type fakeSecrets struct {
	values  map[string]json.RawMessage
	missing bool
}

func (f fakeSecrets) Resolve(_ context.Context, ref string) (json.RawMessage, error) {
	if f.missing {
		return nil, storage.ErrProviderSecretUnavailable
	}
	if value, ok := f.values[ref]; ok {
		return value, nil
	}
	return nil, storage.ErrProviderSecretUnavailable
}

type stubLoader struct {
	records map[uuid.UUID]storage.ProviderRecord
}

func (s stubLoader) LoadProvider(_ context.Context, id uuid.UUID) (*storage.ProviderRecord, error) {
	record, ok := s.records[id]
	if !ok {
		return nil, storage.ErrProviderRecordNotFound
	}
	return &record, nil
}

func providerRecord(id uuid.UUID, secretRef, endpoint string, version int64) storage.ProviderRecord {
	return storage.ProviderRecord{
		ID: id, OrganizationID: uuid.New(), Name: "primary",
		ProviderType: storage.ProviderTypeLocal, Endpoint: endpoint,
		SecretRef: secretRef, ConfigVersion: version, Status: "ACTIVE",
		Capabilities: map[string]any{"encryption": true},
	}
}

func TestRegistryNeverReturnsProviderWithMissingSecret(t *testing.T) {
	id := uuid.New()
	endpoint := t.TempDir()
	registry := storage.NewRegistry(fakeSecrets{missing: true}, stubLoader{records: map[uuid.UUID]storage.ProviderRecord{
		id: providerRecord(id, "vault://missing", endpoint, 1),
	}}, wire.DefaultProviderBuilder)
	_, err := registry.Resolve(context.Background(), id)
	require.ErrorIs(t, err, storage.ErrProviderSecretUnavailable)
}

func TestRegistryBuildsLocalProviderFromEndpointAndSecretRef(t *testing.T) {
	id := uuid.New()
	root := t.TempDir()
	ref := "vault://local-root"
	registry := storage.NewRegistry(fakeSecrets{values: map[string]json.RawMessage{
		ref: json.RawMessage(`{"envelope_key_ref":"kms://org/default"}`),
	}}, stubLoader{records: map[uuid.UUID]storage.ProviderRecord{
		id: providerRecord(id, ref, root, 1),
	}}, wire.DefaultProviderBuilder)
	provider, err := registry.Resolve(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "local", provider.ID())
	body := []byte("registry-smoke")
	_, err = provider.Put(context.Background(), storage.PutRequest{
		ObjectKey: "registry-smoke", Body: bytes.NewReader(body), Size: int64(len(body)),
		SHA256: storage.SHA256(body),
	})
	require.NoError(t, err)
	require.NoError(t, registry.Close())
}

func TestRegistryInvalidatesCacheWhenConfigVersionChanges(t *testing.T) {
	id := uuid.New()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	ref := "vault://local-root"
	var version int64 = 1
	loader := &versionedLoader{records: map[uuid.UUID]storage.ProviderRecord{
		id: providerRecord(id, ref, firstRoot, version),
	}}
	registry := storage.NewRegistry(fakeSecrets{values: map[string]json.RawMessage{
		ref: json.RawMessage(`{}`),
	}}, loader, wire.DefaultProviderBuilder)
	first, err := registry.Resolve(context.Background(), id)
	require.NoError(t, err)
	_, err = first.Put(context.Background(), storage.PutRequest{
		ObjectKey: "only-first", Body: bytes.NewReader([]byte("x")), Size: 1, SHA256: storage.SHA256([]byte("x")),
	})
	require.NoError(t, err)
	loader.records[id] = providerRecord(id, ref, secondRoot, 2)
	second, err := registry.Resolve(context.Background(), id)
	require.NoError(t, err)
	_, err = second.Stat(context.Background(), "only-first")
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.NoError(t, registry.Close())
}

func TestRegistryRejectsInactiveProvider(t *testing.T) {
	id := uuid.New()
	record := providerRecord(id, "vault://local", t.TempDir(), 1)
	record.Status = "DISABLED"
	registry := storage.NewRegistry(fakeSecrets{values: map[string]json.RawMessage{
		"vault://local": json.RawMessage(`{}`),
	}}, stubLoader{records: map[uuid.UUID]storage.ProviderRecord{id: record}}, wire.DefaultProviderBuilder)
	_, err := registry.Resolve(context.Background(), id)
	require.ErrorIs(t, err, storage.ErrProviderInactive)
}

func TestProbeProviderHealthReturnsLatency(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "objects"), 0o750))
	provider, err := local.New(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	health, err := storage.ProbeProviderHealth(context.Background(), provider)
	require.NoError(t, err)
	require.Equal(t, "HEALTHY", health.Status)
	require.GreaterOrEqual(t, health.Latency, time.Duration(0))
	require.False(t, health.CheckedAt.IsZero())
}

type versionedLoader struct {
	records map[uuid.UUID]storage.ProviderRecord
	mu      sync.Mutex
}

func (l *versionedLoader) LoadProvider(_ context.Context, id uuid.UUID) (*storage.ProviderRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	record, ok := l.records[id]
	if !ok {
		return nil, storage.ErrProviderRecordNotFound
	}
	return &record, nil
}
