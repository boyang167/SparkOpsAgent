package nfs

import (
	"context"
	"io"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

type Provider struct {
	inner *local.Provider
}

func New(root string) (*Provider, error) {
	inner, err := local.New(root)
	if err != nil {
		return nil, err
	}
	return &Provider{inner: inner}, nil
}

func (p *Provider) ID() string { return "nfs" }

func (p *Provider) Close() error { return p.inner.Close() }

func (p *Provider) Capabilities() storage.Capabilities {
	return storage.Capabilities{
		RangeRead: true, AtomicPublish: true, VerifiedCopy: true,
		Multipart: false, PresignedUpload: false, POSIX: true,
	}
}

func (p *Provider) Put(ctx context.Context, req storage.PutRequest) (storage.ObjectInfo, error) {
	return p.inner.Put(ctx, req)
}

func (p *Provider) Get(ctx context.Context, req storage.GetRequest) (io.ReadCloser, storage.ObjectInfo, error) {
	return p.inner.Get(ctx, req)
}

func (p *Provider) Stat(ctx context.Context, objectKey string) (storage.ObjectInfo, error) {
	return p.inner.Stat(ctx, objectKey)
}

func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	return p.inner.Delete(ctx, objectKey)
}

func (p *Provider) Copy(ctx context.Context, sourceKey, destinationKey string) (storage.ObjectInfo, error) {
	return p.inner.Copy(ctx, sourceKey, destinationKey)
}
