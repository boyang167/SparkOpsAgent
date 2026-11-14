package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	s3storage "github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/s3"
)

// RuntimeCatalog resolves storage.Provider instances from persisted provider metadata.
type RuntimeCatalog struct {
	repo       repository.FilesystemRepository
	fallback   storage.Provider
	mu         sync.Mutex
	cache      map[uuid.UUID]storage.Provider
	closed     bool
	localRoot  string
	s3Defaults s3storage.Config
}

type RuntimeCatalogConfig struct {
	LocalFallbackRoot string
	S3                s3storage.Config
}

func NewRuntimeCatalog(repo repository.FilesystemRepository, cfg RuntimeCatalogConfig) (*RuntimeCatalog, error) {
	if repo == nil {
		return nil, fmt.Errorf("filesystem runtime catalog requires repository")
	}
	catalog := &RuntimeCatalog{
		repo: repo, cache: make(map[uuid.UUID]storage.Provider),
		localRoot: cfg.LocalFallbackRoot, s3Defaults: cfg.S3,
	}
	if root := strings.TrimSpace(cfg.LocalFallbackRoot); root != "" {
		provider, err := local.New(root)
		if err != nil {
			return nil, err
		}
		catalog.fallback = provider
	}
	return catalog, nil
}

func (c *RuntimeCatalog) Provider(ctx context.Context, id uuid.UUID) (storage.Provider, error) {
	if c == nil {
		return nil, fmt.Errorf("storage provider catalog is not configured")
	}
	if id == uuid.Nil {
		return nil, fmt.Errorf("storage provider id is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, fmt.Errorf("storage provider catalog is closed")
	}
	if provider := c.cache[id]; provider != nil {
		return provider, nil
	}
	meta, err := c.repo.GetStorageProvider(ctx, id)
	if err != nil {
		return nil, err
	}
	provider, err := c.openProvider(meta)
	if err != nil {
		return nil, err
	}
	c.cache[id] = provider
	return provider, nil
}

func (c *RuntimeCatalog) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	providers := make([]storage.Provider, 0, len(c.cache)+1)
	if c.fallback != nil {
		providers = append(providers, c.fallback)
		c.fallback = nil
	}
	for id, provider := range c.cache {
		providers = append(providers, provider)
		delete(c.cache, id)
	}
	c.mu.Unlock()

	var closeErr error
	for _, provider := range providers {
		if closer, ok := provider.(io.Closer); ok {
			closeErr = errors.Join(closeErr, closer.Close())
		}
	}
	return closeErr
}

func (c *RuntimeCatalog) openProvider(meta *model.StorageProvider) (storage.Provider, error) {
	switch meta.ProviderType {
	case model.ProviderTypeLocal:
		root := strings.TrimSpace(meta.Endpoint)
		if root == "" {
			root = c.localRoot
		}
		if root == "" {
			return nil, fmt.Errorf("local provider %s missing endpoint", meta.ID)
		}
		return local.New(root)
	case model.ProviderTypeS3:
		cfg := c.s3Defaults
		if ep := strings.TrimSpace(meta.Endpoint); ep != "" {
			cfg.Endpoint = ep
		}
		if cfg.Bucket == "" {
			return nil, fmt.Errorf("s3 provider %s requires bucket configuration", meta.ID)
		}
		return s3storage.New(cfg)
	case model.ProviderTypeNFS:
		return nil, fmt.Errorf("nfs provider %s is not wired in runtime catalog", meta.ID)
	default:
		return nil, domain.ErrStorageProviderNotFound
	}
}

var _ command.StorageProviderCatalog = (*RuntimeCatalog)(nil)
