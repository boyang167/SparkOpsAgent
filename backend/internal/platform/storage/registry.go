package storage

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrProviderSecretUnavailable = errors.New("storage: provider secret unavailable")
	ErrProviderRecordNotFound    = errors.New("storage: provider record not found")
	ErrProviderInactive          = errors.New("storage: provider inactive")
	ErrUnsupportedProviderType   = errors.New("storage: unsupported provider type")
)

type ProviderType string

const (
	ProviderTypeLocal ProviderType = "LOCAL"
	ProviderTypeNFS   ProviderType = "NFS"
	ProviderTypeS3    ProviderType = "S3"
)

type ProviderRecord struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	ProviderType   ProviderType
	Endpoint       string
	SecretRef      string
	Capabilities   map[string]any
	ConfigVersion  int64
	Status         string
}

type SecretResolver interface {
	Resolve(context.Context, string) (json.RawMessage, error)
}

type ProviderLoader interface {
	LoadProvider(context.Context, uuid.UUID) (*ProviderRecord, error)
}

type ProviderBuilder func(ProviderRecord, json.RawMessage) (Provider, error)

type ProviderHealth struct {
	Status       string
	Latency      time.Duration
	ErrorRate    float64
	CapacityFree int64
	CheckedAt    time.Time
}

type Registry struct {
	secrets SecretResolver
	loader  ProviderLoader
	builder ProviderBuilder
	cache   map[uuid.UUID]cachedProvider
	mu      sync.Mutex
}

type cachedProvider struct {
	version  int64
	provider Provider
}

func NewRegistry(secrets SecretResolver, loader ProviderLoader, builder ProviderBuilder) *Registry {
	if secrets == nil || loader == nil || builder == nil {
		panic("storage registry requires secrets, loader, and builder")
	}
	return &Registry{
		secrets: secrets,
		loader:  loader,
		builder: builder,
		cache:   make(map[uuid.UUID]cachedProvider),
	}
}

func (r *Registry) Resolve(ctx context.Context, id uuid.UUID) (Provider, error) {
	record, err := r.loader.LoadProvider(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.resolveRecord(ctx, record)
}

func (r *Registry) Provider(ctx context.Context, id uuid.UUID) (Provider, error) {
	return r.Resolve(ctx, id)
}

func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var first error
	for id, cached := range r.cache {
		if err := closeProvider(cached.provider); err != nil && first == nil {
			first = err
		}
		delete(r.cache, id)
	}
	return first
}

func (r *Registry) resolveRecord(ctx context.Context, record *ProviderRecord) (Provider, error) {
	if record == nil {
		return nil, ErrProviderRecordNotFound
	}
	if record.Status != "" && record.Status != "ACTIVE" {
		return nil, ErrProviderInactive
	}
	r.mu.Lock()
	if cached, ok := r.cache[record.ID]; ok && cached.version == record.ConfigVersion {
		provider := cached.provider
		r.mu.Unlock()
		return provider, nil
	}
	r.mu.Unlock()

	raw, err := r.secrets.Resolve(ctx, record.SecretRef)
	if err != nil {
		return nil, err
	}
	provider, err := r.builder(*record, raw)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	if cached, ok := r.cache[record.ID]; ok && cached.version != record.ConfigVersion {
		_ = closeProvider(cached.provider)
	}
	r.cache[record.ID] = cachedProvider{version: record.ConfigVersion, provider: provider}
	r.mu.Unlock()
	return provider, nil
}

func closeProvider(provider Provider) error {
	type closer interface {
		Close() error
	}
	if provider == nil {
		return nil
	}
	if c, ok := provider.(closer); ok {
		return c.Close()
	}
	return nil
}

const providerHealthProbeKey = ".sparkops-health-probe"

func ProbeProviderHealth(ctx context.Context, provider Provider) (ProviderHealth, error) {
	if provider == nil {
		return ProviderHealth{}, errors.New("storage: health probe requires provider")
	}
	start := time.Now().UTC()
	_, err := provider.Stat(ctx, providerHealthProbeKey)
	latency := time.Since(start)
	status := "HEALTHY"
	if err != nil && !errors.Is(err, ErrNotFound) {
		status = "UNHEALTHY"
	}
	return ProviderHealth{
		Status:    status,
		Latency:   latency,
		CheckedAt: time.Now().UTC(),
	}, nil
}

func HasEncryptionCapability(capabilities map[string]any) bool {
	if capabilities == nil {
		return false
	}
	value, ok := capabilities["encryption"]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "TRUE"
	default:
		return false
	}
}
