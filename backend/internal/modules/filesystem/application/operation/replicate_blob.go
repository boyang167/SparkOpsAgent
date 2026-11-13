package operation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

const OperationTypeReplicateBlob = "filesystem.replicate-blob.v1"

type ReplicateBlobPayload struct {
	BlobID           string `json:"blob_id"`
	SourceProviderID string `json:"source_provider_id"`
	TargetProviderID string `json:"target_provider_id"`
	PolicyVersion    int64  `json:"policy_version"`
}

type ReplicationConfig struct {
	ChunkSize      int64
	DrainingGrace  time.Duration
}

func DefaultReplicationConfig() ReplicationConfig {
	return ReplicationConfig{ChunkSize: 256, DrainingGrace: 7 * 24 * time.Hour}
}

type replicateRepository interface {
	GetBlob(context.Context, uuid.UUID) (*model.Blob, error)
	GetReplicaByProvider(context.Context, uuid.UUID, uuid.UUID) (*model.StorageReplica, error)
	InsertReplica(context.Context, *model.StorageReplica) error
	UpdateReplica(context.Context, *model.StorageReplica) error
	GetReplicaMigrationCheckpoint(context.Context, uuid.UUID, uuid.UUID) (int64, error)
	SaveReplicaMigrationCheckpoint(context.Context, repository.ReplicaMigrationCheckpoint) error
}

type replicaPromoter interface {
	Handle(context.Context, command.PromoteReplica) error
}

type ReplicateBlobHandler struct {
	repo      replicateRepository
	providers command.StorageProviderCatalog
	promoter  replicaPromoter
	config    ReplicationConfig
}

func NewReplicateBlobHandler(
	repo replicateRepository,
	providers command.StorageProviderCatalog,
	promoter replicaPromoter,
	config ReplicationConfig,
) *ReplicateBlobHandler {
	if config.ChunkSize <= 0 {
		config.ChunkSize = DefaultReplicationConfig().ChunkSize
	}
	if config.DrainingGrace <= 0 {
		config.DrainingGrace = DefaultReplicationConfig().DrainingGrace
	}
	return &ReplicateBlobHandler{repo: repo, providers: providers, promoter: promoter, config: config}
}

func (h *ReplicateBlobHandler) Handle(ctx context.Context, operation messaging.Operation) error {
	var payload ReplicateBlobPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return messaging.Permanent(fmt.Errorf("decode replicate blob operation: %w", err))
	}
	return h.Execute(ctx, payload)
}

func (h *ReplicateBlobHandler) Execute(ctx context.Context, payload ReplicateBlobPayload) error {
	if h.repo == nil || h.providers == nil || h.promoter == nil {
		return fmt.Errorf("replicate blob handler dependencies are required")
	}
	blobID, err := uuid.Parse(strings.TrimSpace(payload.BlobID))
	if err != nil {
		return messaging.Permanent(err)
	}
	sourceProviderID, err := uuid.Parse(strings.TrimSpace(payload.SourceProviderID))
	if err != nil {
		return messaging.Permanent(err)
	}
	targetProviderID, err := uuid.Parse(strings.TrimSpace(payload.TargetProviderID))
	if err != nil {
		return messaging.Permanent(err)
	}

	blob, err := h.repo.GetBlob(ctx, blobID)
	if err != nil {
		return err
	}
	sourceReplica, err := h.repo.GetReplicaByProvider(ctx, blobID, sourceProviderID)
	if err != nil {
		return err
	}
	if sourceReplica.State != model.ReplicaStateAvailable {
		return fmt.Errorf("source replica must be available")
	}

	targetReplica, targetKey, err := h.ensureTargetReplica(ctx, blob, sourceProviderID, targetProviderID, payload.PolicyVersion)
	if err != nil {
		return err
	}

	checkpoint, err := h.repo.GetReplicaMigrationCheckpoint(ctx, blobID, targetProviderID)
	if err != nil {
		return err
	}

	sourceProvider, err := h.providers.Provider(ctx, sourceProviderID)
	if err != nil {
		return err
	}
	targetProvider, err := h.providers.Provider(ctx, targetProviderID)
	if err != nil {
		return err
	}

	if checkpoint < blob.Size {
		if err := h.copyFromCheckpoint(ctx, blob, sourceReplica, targetReplica, targetKey, checkpoint, sourceProvider, targetProvider); err != nil {
			return err
		}
	}

	targetReplica.State = model.ReplicaStateVerifying
	if err := h.repo.UpdateReplica(ctx, targetReplica); err != nil {
		return err
	}
	if err := h.verifyReplica(ctx, blob, targetProvider, targetKey); err != nil {
		targetReplica.State = model.ReplicaStateFailed
		_ = h.repo.UpdateReplica(ctx, targetReplica)
		return err
	}

	verifiedAt := time.Now().UTC()
	targetReplica.State = model.ReplicaStateAvailable
	targetReplica.Checksum = blob.SHA256
	targetReplica.VerifiedAt = &verifiedAt
	if err := h.repo.UpdateReplica(ctx, targetReplica); err != nil {
		return err
	}

	return h.promoter.Handle(ctx, command.PromoteReplica{
		BlobID: blobID, SourceProviderID: sourceProviderID, TargetProviderID: targetProviderID,
		DrainingGrace: h.config.DrainingGrace,
	})
}

func (h *ReplicateBlobHandler) ensureTargetReplica(
	ctx context.Context,
	blob *model.Blob,
	sourceProviderID, targetProviderID uuid.UUID,
	policyVersion int64,
) (*model.StorageReplica, string, error) {
	blobID := blob.ID
	targetKey := fmt.Sprintf("replica-migrations/%s/%s", blobID, targetProviderID)
	targetReplica, err := h.repo.GetReplicaByProvider(ctx, blobID, targetProviderID)
	if err == nil {
		return targetReplica, targetReplica.ObjectKey, nil
	}

	replicaID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(targetKey))
	targetReplica = &model.StorageReplica{
		ID: replicaID, BlobID: blobID, ProviderID: targetProviderID,
		ObjectKey: targetKey, State: model.ReplicaStateCreating,
	}
	if err := h.repo.InsertReplica(ctx, targetReplica); err != nil {
		return nil, "", err
	}
	if err := h.repo.SaveReplicaMigrationCheckpoint(ctx, repository.ReplicaMigrationCheckpoint{
		BlobID: blobID, TargetProviderID: targetProviderID, TargetReplicaID: replicaID,
		SourceProviderID: sourceProviderID, BytesCopied: 0, TargetObjectKey: targetKey, PolicyVersion: policyVersion,
	}); err != nil {
		return nil, "", err
	}
	return targetReplica, targetKey, nil
}

func (h *ReplicateBlobHandler) copyFromCheckpoint(
	ctx context.Context,
	blob *model.Blob,
	sourceReplica, targetReplica *model.StorageReplica,
	targetKey string,
	checkpoint int64,
	sourceProvider, targetProvider storage.Provider,
) error {
	existing := make([]byte, 0, blob.Size)
	if checkpoint > 0 {
		sourceReader, _, readErr := sourceProvider.Get(ctx, storage.GetRequest{
			ObjectKey: sourceReplica.ObjectKey, Offset: 0, Length: checkpoint,
		})
		if readErr != nil {
			return readErr
		}
		var err error
		existing, err = readExactly(sourceReader, checkpoint)
		_ = sourceReader.Close()
		if err != nil {
			return err
		}
	}

	for offset := checkpoint; offset < blob.Size; {
		chunk := h.config.ChunkSize
		if remain := blob.Size - offset; remain < chunk {
			chunk = remain
		}
		reader, _, err := sourceProvider.Get(ctx, storage.GetRequest{
			ObjectKey: sourceReplica.ObjectKey, Offset: offset, Length: chunk,
		})
		if err != nil {
			return err
		}
		chunkData, err := readExactly(reader, chunk)
		_ = reader.Close()
		if err != nil {
			return err
		}
		existing = append(existing, chunkData...)
		sum := sha256.Sum256(existing)
		body := bytes.NewReader(existing)
		if err := targetProvider.Delete(ctx, targetKey); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return err
		}
		_, err = targetProvider.Put(ctx, storage.PutRequest{
			ObjectKey: targetKey,
			Body:      body,
			Size:      int64(len(existing)),
			SHA256:    hex.EncodeToString(sum[:]),
		})
		if err != nil {
			if restoreErr := h.restorePartialObject(ctx, targetProvider, targetKey, existing[:offset]); restoreErr != nil {
				return restoreErr
			}
			if saveErr := h.saveCheckpoint(ctx, blob.ID, sourceReplica.ProviderID, targetReplica, targetKey, offset); saveErr != nil {
				return saveErr
			}
			return err
		}
		offset = int64(len(existing))
		if err := h.saveCheckpoint(ctx, blob.ID, sourceReplica.ProviderID, targetReplica, targetKey, offset); err != nil {
			return err
		}
	}
	targetReplica.ObjectKey = targetKey
	if err := h.repo.UpdateReplica(ctx, targetReplica); err != nil {
		return err
	}
	return nil
}

func (h *ReplicateBlobHandler) verifyReplica(ctx context.Context, blob *model.Blob, provider storage.Provider, objectKey string) error {
	reader, _, err := provider.Get(ctx, storage.GetRequest{ObjectKey: objectKey, Offset: 0, Length: blob.Size})
	if err != nil {
		return err
	}
	defer reader.Close()
	hasher := sha256.New()
	written, err := io.Copy(hasher, io.LimitReader(reader, blob.Size))
	if err != nil {
		return err
	}
	if written != blob.Size {
		return storage.ErrSizeMismatch
	}
	got := strings.ToLower(hex.EncodeToString(hasher.Sum(nil)))
	want := strings.ToLower(blob.SHA256)
	if got != want {
		return storage.ErrChecksumMismatch
	}
	return nil
}

func readExactly(reader io.Reader, size int64) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (h *ReplicateBlobHandler) restorePartialObject(ctx context.Context, provider storage.Provider, objectKey string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	sum := sha256.Sum256(data)
	_, err := provider.Put(ctx, storage.PutRequest{
		ObjectKey: objectKey,
		Body:      bytes.NewReader(data),
		Size:      int64(len(data)),
		SHA256:    hex.EncodeToString(sum[:]),
	})
	return err
}

func (h *ReplicateBlobHandler) saveCheckpoint(
	ctx context.Context,
	blobID, sourceProviderID uuid.UUID,
	targetReplica *model.StorageReplica,
	targetKey string,
	bytesCopied int64,
) error {
	return h.repo.SaveReplicaMigrationCheckpoint(ctx, repository.ReplicaMigrationCheckpoint{
		BlobID: blobID, TargetProviderID: targetReplica.ProviderID, TargetReplicaID: targetReplica.ID,
		SourceProviderID: sourceProviderID, BytesCopied: bytesCopied,
		TargetObjectKey: targetKey, PolicyVersion: 0,
	})
}
