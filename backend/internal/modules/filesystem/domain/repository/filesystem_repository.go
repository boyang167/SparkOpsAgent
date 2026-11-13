package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

var (
	ErrNodeNameConflict = domain.ErrNodeNameConflict
	ErrNodeNotFound     = domain.ErrNodeNotFound
	ErrTransferNotFound = domain.ErrNodeNotFound
	ErrTransferConflict = domain.ErrRevisionConflict
)

type CommandDedup struct {
	CommandID   uuid.UUID
	WorkspaceID uuid.UUID
	Operation   string
	RequestHash string
	NodeID      *uuid.UUID
	RevisionID  *uuid.UUID
	State       string
}

var (
	ErrCommandInProgress   = domain.ErrCommandInProgress
	ErrIdempotencyConflict = domain.ErrIdempotencyConflict
)

type FilesystemRepository interface {
	CreateNamespace(context.Context, *model.Namespace) error
	GetNamespaceByWorkspace(context.Context, uuid.UUID) (*model.Namespace, error)
	SetNamespaceRoot(context.Context, uuid.UUID, uuid.UUID) error
	CreateNode(context.Context, *model.Node) error
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	GetNodeIncludingTrashed(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	ListChildren(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error)
	NextRevisionNumber(context.Context, uuid.UUID) (int, error)
	SaveRevision(context.Context, *model.FileRevision, *model.Blob, *model.StorageReplica) error
	AcquireBlob(context.Context, uuid.UUID, string, int64) (*model.Blob, error)
	GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error)
	ListRevisions(context.Context, uuid.UUID, uuid.UUID, int, string) ([]model.FileRevision, string, error)
	SetCurrentRevision(context.Context, uuid.UUID, uuid.UUID) error
	IncrementBlobRef(context.Context, uuid.UUID) error
	GetBlob(context.Context, uuid.UUID) (*model.Blob, error)
	GetAvailableReplica(context.Context, uuid.UUID, uuid.UUID) (*model.StorageReplica, error)
	ListResourceGrants(context.Context, uuid.UUID, uuid.UUID, model.ResourcePermission, time.Time) ([]model.ResourceGrant, error)
	CreateResourceGrant(context.Context, model.ResourceGrant) error
	RevokeResourceGrant(context.Context, uuid.UUID, uuid.UUID) error
	ListNodeResourceGrants(context.Context, uuid.UUID) ([]model.ResourceGrant, error)
	ApplyQuotaUsage(context.Context, uuid.UUID, int64, int64) error
	RenameNode(context.Context, uuid.UUID, string) error
	MoveNodeSafely(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, string) error
	TrashNode(context.Context, uuid.UUID) error
	ClaimCommand(context.Context, CommandDedup) (*CommandDedup, error)
	CompleteCommand(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) error
	GetCommand(context.Context, uuid.UUID) (*CommandDedup, error)
	CreateTransfer(context.Context, *model.TransferSession) error
	GetTransferByID(context.Context, uuid.UUID) (*model.TransferSession, error)
	GetTransferByIdempotency(context.Context, uuid.UUID, string) (*model.TransferSession, error)
	CASTransferStatus(context.Context, uuid.UUID, model.TransferStatus, model.TransferStatus, int64) error
	LockTransfer(context.Context, uuid.UUID) (*model.TransferSession, error)
	ListTransferParts(context.Context, uuid.UUID) ([]model.TransferPart, error)
	UpsertTransferPart(context.Context, *model.TransferPart) error
}
