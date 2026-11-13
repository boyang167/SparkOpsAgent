package operation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

const OperationTypeGeneratePreview = "filesystem.generate-preview.v1"

type GeneratePreviewPayload struct {
	WorkspaceID    string `json:"workspace_id"`
	FileID         string `json:"file_id"`
	RevisionID     string `json:"revision_id"`
	Classification string `json:"classification"`
}

type generatePreviewRepository interface {
	GetNamespace(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNodeByID(context.Context, uuid.UUID) (*model.Node, error)
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error)
	GetPreferredReplica(context.Context, uuid.UUID) (*model.StorageReplica, error)
	AcquireBlob(context.Context, uuid.UUID, string, int64) (*model.Blob, error)
	InsertReplica(context.Context, *model.StorageReplica) error
	IncrementBlobRef(context.Context, uuid.UUID) error
	CreateFileLink(context.Context, model.FileLink) error
	GetFileLink(context.Context, uuid.UUID, *uuid.UUID, string) (*model.FileLink, error)
}

type GeneratePreviewHandler struct {
	repo      generatePreviewRepository
	providers command.StorageProviderCatalog
	previewer port.Previewer
}

func NewGeneratePreviewHandler(
	repo generatePreviewRepository,
	providers command.StorageProviderCatalog,
	previewer port.Previewer,
) *GeneratePreviewHandler {
	if previewer == nil {
		previewer = port.NoopPreviewer{}
	}
	return &GeneratePreviewHandler{repo: repo, providers: providers, previewer: previewer}
}

func (h *GeneratePreviewHandler) Handle(ctx context.Context, operation messaging.Operation) error {
	var payload GeneratePreviewPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return messaging.Permanent(fmt.Errorf("decode generate preview operation: %w", err))
	}
	return h.Execute(ctx, payload)
}

func (h *GeneratePreviewHandler) Execute(ctx context.Context, payload GeneratePreviewPayload) error {
	if h == nil || h.repo == nil {
		return fmt.Errorf("generate preview handler dependencies are required")
	}
	fileID, err := uuid.Parse(strings.TrimSpace(payload.FileID))
	if err != nil {
		return messaging.Permanent(err)
	}
	revisionID, err := uuid.Parse(strings.TrimSpace(payload.RevisionID))
	if err != nil {
		return messaging.Permanent(err)
	}
	node, err := h.repo.GetNodeByID(ctx, fileID)
	if err != nil {
		return err
	}
	ns, err := h.repo.GetNamespace(ctx, node.NamespaceID)
	if err != nil {
		return err
	}
	if node.Type != model.NodeTypeFile {
		return messaging.Permanent(domain.ErrUnsupportedNodeType)
	}
	revision, err := h.repo.GetRevision(ctx, fileID, revisionID)
	if err != nil {
		return err
	}
	existing, existingErr := h.repo.GetFileLink(ctx, fileID, &revisionID, model.FileLinkTypePreview)
	if existingErr == nil && existing != nil {
		return nil
	}
	if h.providers == nil {
		return messaging.Permanent(fmt.Errorf("generate preview requires storage providers"))
	}
	replica, err := h.repo.GetPreferredReplica(ctx, revision.BlobID())
	if err != nil {
		return err
	}
	provider, err := h.providers.Provider(ctx, replica.ProviderID)
	if err != nil {
		return err
	}
	reader, _, err := provider.Get(ctx, storage.GetRequest{ObjectKey: replica.ObjectKey})
	if err != nil {
		return err
	}
	defer reader.Close()
	content, readErr := io.ReadAll(io.LimitReader(reader, revision.Size()+1))
	if readErr != nil {
		return readErr
	}
	if int64(len(content)) != revision.Size() {
		return fmt.Errorf("preview source size mismatch")
	}
	classification := strings.TrimSpace(payload.Classification)
	if classification == "" {
		classification = string(model.TransferClassificationInternal)
	}
	rendered, err := h.previewer.Render(ctx, port.PreviewRequest{
		Source: port.PreviewSource{
			MIMEType: revision.MIMEType(), Size: revision.Size(), Bytes: content,
			Object: port.ObjectRef{ProviderID: replica.ProviderID, ObjectKey: replica.ObjectKey},
		},
		Classification: classification,
	})
	if err != nil {
		return messaging.Permanent(err)
	}
	if len(rendered.Bytes) == 0 {
		return messaging.Permanent(port.ErrPreviewUnsupported)
	}
	sha := strings.ToLower(strings.TrimSpace(rendered.SHA256))
	if sha == "" {
		sum := sha256.Sum256(rendered.Bytes)
		sha = hex.EncodeToString(sum[:])
	}
	size := rendered.Size
	if size == 0 {
		size = int64(len(rendered.Bytes))
	}
	mimeType := strings.TrimSpace(rendered.MIMEType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	objectKey := fmt.Sprintf("previews/%s/%s/%s", ns.OrganizationID, fileID, revisionID)
	blob, err := h.repo.AcquireBlob(ctx, ns.OrganizationID, sha, size)
	if err != nil {
		return err
	}
	if _, putErr := provider.Put(ctx, storage.PutRequest{ObjectKey: objectKey, Body: bytes.NewReader(rendered.Bytes), Size: size, SHA256: sha}); putErr != nil && !errorsIsConflict(putErr) {
		return putErr
	}
	now := time.Now().UTC()
	replicaID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-preview:"+revisionID.String()))
	if err := h.repo.InsertReplica(ctx, &model.StorageReplica{
		ID: replicaID, BlobID: blob.ID, ProviderID: replica.ProviderID, ObjectKey: objectKey,
		Checksum: sha, State: model.ReplicaStateAvailable, VerifiedAt: &now,
	}); err != nil && !errorsIsConflict(err) {
		return err
	}
	if err := h.repo.IncrementBlobRef(ctx, blob.ID); err != nil {
		return err
	}
	return h.repo.CreateFileLink(ctx, model.FileLink{
		ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-preview-link:"+revisionID.String())),
		NodeID: fileID, RevisionID: &revisionID, LinkType: model.FileLinkTypePreview,
		LinkID: blob.ID, CreatedAt: now,
	})
}

func errorsIsConflict(err error) bool {
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "conflict") || err == storage.ErrObjectConflict)
}
