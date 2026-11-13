package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

const OperationTypeGarbageCollect = "filesystem.garbage-collect.v1"

type GarbageCollectPayload struct {
	BlobID string `json:"blob_id"`
}

type GarbageCollectConfig struct {
	GracePeriod time.Duration
}

func DefaultGarbageCollectConfig() GarbageCollectConfig {
	return GarbageCollectConfig{GracePeriod: 7 * 24 * time.Hour}
}

type gcRepository interface {
	ConfirmBlobGCEligible(context.Context, uuid.UUID, time.Duration, time.Time) (repository.BlobGCContext, error)
	ListReplicasForGC(context.Context, uuid.UUID) ([]model.StorageReplica, error)
	FinalizeBlobGC(context.Context, uuid.UUID) error
	MarkBlobGCPending(context.Context, uuid.UUID) error
	GetBlob(context.Context, uuid.UUID) (*model.Blob, error)
	GetReplicaByProvider(context.Context, uuid.UUID, uuid.UUID) (*model.StorageReplica, error)
}

type GarbageCollector struct {
	repo      gcRepository
	providers command.StorageProviderCatalog
	config    GarbageCollectConfig
	clock     func() time.Time
}

func NewGarbageCollector(repo gcRepository, providers command.StorageProviderCatalog, config GarbageCollectConfig) *GarbageCollector {
	if config.GracePeriod <= 0 {
		config = DefaultGarbageCollectConfig()
	}
	return &GarbageCollector{
		repo: repo, providers: providers, config: config,
		clock: func() time.Time { return time.Now().UTC() },
	}
}

func (g *GarbageCollector) Handle(ctx context.Context, operation messaging.Operation) error {
	var payload GarbageCollectPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return messaging.Permanent(fmt.Errorf("decode garbage collect operation: %w", err))
	}
	blobID, err := uuid.Parse(strings.TrimSpace(payload.BlobID))
	if err != nil {
		return messaging.Permanent(err)
	}
	return g.Delete(ctx, blobID)
}

func (g *GarbageCollector) Delete(ctx context.Context, blobID uuid.UUID) error {
	if g.repo == nil || g.providers == nil {
		return fmt.Errorf("garbage collector dependencies are required")
	}
	now := g.clock()
	if _, err := g.repo.ConfirmBlobGCEligible(ctx, blobID, g.config.GracePeriod, now); err != nil {
		return err
	}
	replicas, err := g.repo.ListReplicasForGC(ctx, blobID)
	if err != nil {
		return err
	}
	for _, replica := range replicas {
		provider, providerErr := g.providers.Provider(ctx, replica.ProviderID)
		if providerErr != nil {
			_ = g.repo.MarkBlobGCPending(ctx, blobID)
			return providerErr
		}
		if deleteErr := provider.Delete(ctx, replica.ObjectKey); deleteErr != nil {
			_ = g.repo.MarkBlobGCPending(ctx, blobID)
			return deleteErr
		}
	}
	return g.repo.FinalizeBlobGC(ctx, blobID)
}
