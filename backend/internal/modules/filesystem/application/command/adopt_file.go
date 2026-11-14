package command

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const AdoptionLineageRelation = "ADOPTED_AS_ORGANIZATION_FILE"

type AdoptionAuthorizationRequest struct {
	ActorID     uuid.UUID
	WorkspaceID uuid.UUID
	NamespaceID uuid.UUID
	NodeID      uuid.UUID
	Permission  model.ResourcePermission
}

type AdoptionAuthorizer interface {
	Authorize(context.Context, AdoptionAuthorizationRequest) bool
}

type AdoptionStore interface {
	GetNodeByID(context.Context, uuid.UUID) (*model.Node, error)
	GetNamespace(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNamespaceByOrganization(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error)
	GetBlob(context.Context, uuid.UUID) (*model.Blob, error)
	GetAvailableReplica(context.Context, uuid.UUID, uuid.UUID) (*model.StorageReplica, error)
	CreateNode(context.Context, *model.Node) error
	SaveRevision(context.Context, *model.FileRevision, *model.Blob, *model.StorageReplica) error
	SetCurrentRevision(context.Context, uuid.UUID, uuid.UUID) error
	IncrementBlobRef(context.Context, uuid.UUID) error
	ApplyNamespaceQuotaUsage(context.Context, uuid.UUID, int64, int64) error
	CreateFileLineage(context.Context, model.FileLineage) error
	ClaimCommand(context.Context, repository.CommandDedup) (*repository.CommandDedup, error)
	CompleteCommand(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) error
}

type AdoptFile struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	SourceNodeID   uuid.UUID
	TargetParentID uuid.UUID
	Name           string
	TraceID        string
}

type AdoptFileHandler struct {
	store      AdoptionStore
	tx         postgres.Transactor
	authorizer AdoptionAuthorizer
	audit      auditPort
	now        func() time.Time
}

func NewAdoptFileHandler(store AdoptionStore, tx postgres.Transactor, authorizer AdoptionAuthorizer, logger auditPort) *AdoptFileHandler {
	if store == nil || tx == nil || authorizer == nil {
		panic("filesystem adoption dependencies are required")
	}
	return &AdoptFileHandler{store: store, tx: tx, authorizer: authorizer, audit: logger, now: func() time.Time { return time.Now().UTC() }}
}

func (h *AdoptFileHandler) Handle(ctx context.Context, cmd AdoptFile) (*model.Node, error) {
	if cmd.CommandID == uuid.Nil || cmd.ActorID == uuid.Nil || cmd.OrganizationID == uuid.Nil ||
		cmd.SourceNodeID == uuid.Nil || cmd.TargetParentID == uuid.Nil || strings.TrimSpace(cmd.Name) == "" {
		return nil, domain.ErrIdempotencyConflict
	}
	source, err := h.store.GetNodeByID(ctx, cmd.SourceNodeID)
	if err != nil {
		return nil, err
	}
	if source.Type != model.NodeTypeFile || source.State != model.NodeStateActive || source.CurrentRevisionID == nil {
		return nil, domain.ErrUnsupportedNodeType
	}
	sourceNamespace, err := h.store.GetNamespace(ctx, source.NamespaceID)
	if err != nil {
		return nil, err
	}
	targetNamespace, err := h.store.GetNamespaceByOrganization(ctx, cmd.OrganizationID)
	if err != nil {
		return nil, err
	}
	if sourceNamespace.OrganizationID != cmd.OrganizationID || targetNamespace.OrganizationID != cmd.OrganizationID {
		return nil, domain.ErrCrossOrganizationGrant
	}
	if targetNamespace.NamespaceType != model.NamespaceTypeOrganization || targetNamespace.Status != "" && targetNamespace.Status != "ACTIVE" {
		return nil, domain.ErrNamespaceMismatch
	}
	parent, err := h.store.GetNode(ctx, targetNamespace.ID, cmd.TargetParentID)
	if err != nil {
		return nil, err
	}
	if parent.Type != model.NodeTypeDirectory || parent.State != model.NodeStateActive {
		return nil, domain.ErrParentNotFound
	}
	if !h.authorizer.Authorize(ctx, AdoptionAuthorizationRequest{
		ActorID: cmd.ActorID, WorkspaceID: cmd.WorkspaceID, NamespaceID: source.NamespaceID,
		NodeID: source.ID, Permission: model.ResourcePermissionRead,
	}) || !h.authorizer.Authorize(ctx, AdoptionAuthorizationRequest{
		ActorID: cmd.ActorID, WorkspaceID: cmd.WorkspaceID, NamespaceID: targetNamespace.ID,
		NodeID: parent.ID, Permission: model.ResourcePermissionWrite,
	}) {
		return nil, domain.ErrResourceDenied
	}

	targetID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-adoption-node:"+cmd.CommandID.String()))
	revisionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-adoption-revision:"+cmd.CommandID.String()))
	requestHash := mutationRequestHash(
		"adopt", cmd.OrganizationID, cmd.WorkspaceID, source.NamespaceID, source.ID,
		*source.CurrentRevisionID, targetNamespace.ID, parent.ID, strings.TrimSpace(cmd.Name), cmd.ActorID,
	)
	var target *model.Node
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		claimed, claimErr := h.store.ClaimCommand(txCtx, repository.CommandDedup{
			CommandID: cmd.CommandID, WorkspaceID: cmd.OrganizationID, Operation: "adopt",
			RequestHash: requestHash, NodeID: &targetID, RevisionID: &revisionID,
		})
		if claimErr != nil {
			return claimErr
		}
		if claimed.State == "COMPLETED" {
			if claimed.NodeID == nil || claimed.RevisionID == nil {
				return domain.ErrOutcomeUnknown
			}
			target, claimErr = h.store.GetNode(txCtx, targetNamespace.ID, *claimed.NodeID)
			return claimErr
		}
		sourceRevision, getErr := h.store.GetRevision(txCtx, source.ID, *source.CurrentRevisionID)
		if getErr != nil {
			return getErr
		}
		blob, getErr := h.store.GetBlob(txCtx, sourceRevision.BlobID())
		if getErr != nil {
			return getErr
		}
		if blob.State != model.BlobStateAvailable || len(blob.SHA256) != 64 || blob.Size != sourceRevision.Size() {
			return domain.ErrTransferIntegrity
		}
		replica, getErr := h.store.GetAvailableReplica(txCtx, blob.ID, uuid.Nil)
		if getErr != nil {
			return getErr
		}
		target, getErr = model.NewFile(targetNamespace.ID, cmd.ActorID, strings.TrimSpace(cmd.Name), parent.ID)
		if getErr != nil {
			return getErr
		}
		target.ID = targetID
		target.Classification = source.Classification
		if createErr := h.store.CreateNode(txCtx, target); createErr != nil {
			return createErr
		}
		targetRevision, revisionErr := model.NewRevisionWithID(
			revisionID, target.ID, 1, blob.ID, sourceRevision.Size(), sourceRevision.MIMEType(), cmd.ActorID,
		)
		if revisionErr != nil {
			return revisionErr
		}
		if saveErr := h.store.SaveRevision(txCtx, targetRevision, blob, replica); saveErr != nil {
			return saveErr
		}
		if setErr := h.store.SetCurrentRevision(txCtx, target.ID, revisionID); setErr != nil {
			return setErr
		}
		if refErr := h.store.IncrementBlobRef(txCtx, blob.ID); refErr != nil {
			return refErr
		}
		if quotaErr := h.store.ApplyNamespaceQuotaUsage(txCtx, targetNamespace.ID, sourceRevision.Size(), 1); quotaErr != nil {
			return quotaErr
		}
		lineage := model.FileLineage{
			ID:             uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-adoption-lineage:"+cmd.CommandID.String())),
			OrganizationID: cmd.OrganizationID, SourceNamespaceID: source.NamespaceID,
			SourceNodeID: source.ID, SourceRevisionID: *source.CurrentRevisionID,
			TargetNamespaceID: targetNamespace.ID, TargetNodeID: target.ID, TargetRevisionID: revisionID,
			Relation: AdoptionLineageRelation, CommandID: cmd.CommandID, CreatedBy: cmd.ActorID, CreatedAt: h.now(),
		}
		if lineageErr := h.store.CreateFileLineage(txCtx, lineage); lineageErr != nil {
			return lineageErr
		}
		if completeErr := h.store.CompleteCommand(txCtx, cmd.CommandID, &target.ID, &revisionID); completeErr != nil {
			return completeErr
		}
		if h.audit != nil {
			return h.audit.Log(txCtx, audit.Entry{
				ActorID: kernel.MemberID{UUID: cmd.ActorID}, OrgID: &kernel.OrgID{UUID: cmd.OrganizationID},
				Action: "filesystem.file.adopted", ResourceType: "filesystem", ResourceID: target.ID.String(),
				Detail: map[string]any{
					"namespace_id": targetNamespace.ID.String(), "node_id": target.ID.String(),
					"principal_id":        cmd.ActorID.String(),
					"source_namespace_id": source.NamespaceID.String(), "source_node_id": source.ID.String(),
					"principal_before": map[string]any{"owner_id": source.OwnerID.String()},
					"principal_after":  map[string]any{"owner_id": cmd.ActorID.String()},
					"reason":           "adopt_as_organization_file", "trace_id": cmd.TraceID,
					"action": "ADOPT", "decision": "ALLOW",
				},
			})
		}
		return nil
	})
	return target, err
}
