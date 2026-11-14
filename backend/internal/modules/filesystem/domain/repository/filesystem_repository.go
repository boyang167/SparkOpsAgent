package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

var (
	ErrNodeNameConflict            = domain.ErrNodeNameConflict
	ErrNodeNotFound                = domain.ErrNodeNotFound
	ErrTransferNotFound            = domain.ErrTransferNotFound
	ErrTransferConflict            = domain.ErrTransferConflict
	ErrTransferIdempotencyConflict = domain.ErrTransferIdempotencyConflict
	ErrTransferVersionConflict     = domain.ErrTransferVersionConflict
	ErrTransferStatusConflict      = domain.ErrTransferStatusConflict
	ErrTransferPartConflict        = domain.ErrTransferPartConflict
	ErrTransferPartOverlap         = domain.ErrTransferPartOverlap
	ErrTransferIntegrity           = domain.ErrTransferIntegrity
	ErrTransferExpired             = domain.ErrTransferExpired
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

type TransferStatePatch struct {
	StatusFrom   model.TransferStatus
	StatusTo     model.TransferStatus
	ActualSize   *int64
	ActualSHA256 string
	ProviderID   *uuid.UUID
	ScanResult   *model.TransferScanResult
	ErrorCode    *string
}

type ReplicaMigrationCheckpoint struct {
	BlobID           uuid.UUID
	TargetProviderID uuid.UUID
	TargetReplicaID  uuid.UUID
	SourceProviderID uuid.UUID
	BytesCopied      int64
	TargetObjectKey  string
	PolicyVersion    int64
}

type ReplicaPromotion struct {
	BlobID           uuid.UUID
	SourceProviderID uuid.UUID
	TargetProviderID uuid.UUID
	DeleteAfter      time.Time
}

type QuotaReservation struct {
	ID             uuid.UUID
	QuotaAccountID uuid.UUID
	WorkspaceID    uuid.UUID
	SessionID      uuid.UUID
	Bytes          int64
	Files          int64
	State          string
	ExpiresAt      time.Time
}

var (
	ErrCommandInProgress   = domain.ErrCommandInProgress
	ErrIdempotencyConflict = domain.ErrIdempotencyConflict
)

type FilesystemRepository interface {
	CreateNamespace(context.Context, *model.Namespace) error
	GetNamespaceByOrganization(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNamespaceByUser(context.Context, uuid.UUID, uuid.UUID) (*model.Namespace, error)
	ListNamespacesForOrganization(context.Context, uuid.UUID) ([]*model.Namespace, error)
	GetNamespaceByWorkspace(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNamespace(context.Context, uuid.UUID) (*model.Namespace, error)
	SetNamespaceRoot(context.Context, uuid.UUID, uuid.UUID) error
	CreateNode(context.Context, *model.Node) error
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	GetNodeIncludingTrashed(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	ListChildren(context.Context, uuid.UUID, *uuid.UUID, *model.NodeType, int, string) ([]model.Node, string, error)
	NextRevisionNumber(context.Context, uuid.UUID) (int, error)
	SaveRevision(context.Context, *model.FileRevision, *model.Blob, *model.StorageReplica) error
	AcquireBlob(context.Context, uuid.UUID, string, int64) (*model.Blob, error)
	GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error)
	ListRevisions(context.Context, uuid.UUID, uuid.UUID, int, string) ([]model.FileRevision, string, error)
	SetCurrentRevision(context.Context, uuid.UUID, uuid.UUID) error
	IncrementBlobRef(context.Context, uuid.UUID) error
	GetBlob(context.Context, uuid.UUID) (*model.Blob, error)
	GetAvailableReplica(context.Context, uuid.UUID, uuid.UUID) (*model.StorageReplica, error)
	GetPreferredReplica(context.Context, uuid.UUID) (*model.StorageReplica, error)
	GetReplicaByProvider(context.Context, uuid.UUID, uuid.UUID) (*model.StorageReplica, error)
	InsertBlob(context.Context, *model.Blob) error
	InsertReplica(context.Context, *model.StorageReplica) error
	SetPreferredProvider(context.Context, uuid.UUID, uuid.UUID) error
	UpdateReplica(context.Context, *model.StorageReplica) error
	GetReplicaMigrationCheckpoint(context.Context, uuid.UUID, uuid.UUID) (int64, error)
	SaveReplicaMigrationCheckpoint(context.Context, ReplicaMigrationCheckpoint) error
	PromoteReplica(context.Context, ReplicaPromotion) error
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
	ReserveTransferQuota(context.Context, uuid.UUID, int64, int64) (uuid.UUID, error)
	CreateQuotaReservation(context.Context, QuotaReservation) error
	CommitQuotaReservation(context.Context, uuid.UUID, uuid.UUID) error
	ReleaseQuotaReservation(context.Context, uuid.UUID, uuid.UUID) error
	GetTransferByID(context.Context, uuid.UUID) (*model.TransferSession, error)
	GetTransferByIdempotency(context.Context, uuid.UUID, string) (*model.TransferSession, error)
	ListTransfers(context.Context, uuid.UUID, *model.TransferStatus, int, string) ([]*model.TransferSession, string, error)
	LockTransfer(context.Context, uuid.UUID) (*model.TransferSession, error)
	ListTransferParts(context.Context, uuid.UUID) ([]model.TransferPart, error)
	UpsertTransferPart(context.Context, *model.TransferPart) error
	SaveTransferStateCAS(context.Context, uuid.UUID, uuid.UUID, int64, TransferStatePatch) error
	CreateStorageProvider(context.Context, *model.StorageProvider) error
	GetStorageProvider(context.Context, uuid.UUID) (*model.StorageProvider, error)
	ListStorageProviders(context.Context, uuid.UUID) ([]*model.StorageProvider, error)
	UpdateStorageProvider(context.Context, *model.StorageProvider) error
	DeleteStorageProvider(context.Context, uuid.UUID, uuid.UUID) error
	SaveStoragePool(context.Context, *model.StoragePool, uuid.UUID) error
	GetStoragePool(context.Context, uuid.UUID) (*model.StoragePool, error)
	GetStoragePoolForOrganization(context.Context, uuid.UUID, uuid.UUID) (*model.StoragePool, error)
	ListStoragePools(context.Context, uuid.UUID) ([]model.StoragePoolSnapshot, error)
	DeleteStoragePool(context.Context, uuid.UUID, uuid.UUID) error
	SaveStorageBinding(context.Context, *model.StorageBinding) error
	ListStorageBindings(context.Context, uuid.UUID) ([]model.StorageBinding, error)
	SaveLifecyclePolicy(context.Context, *model.LifecyclePolicy) error
	ListLifecyclePolicies(context.Context, uuid.UUID) ([]model.LifecyclePolicy, error)
	CreateLifecycleJob(context.Context, *model.LifecycleJob) error
	GetNodeByID(context.Context, uuid.UUID) (*model.Node, error)
	CreateArtifact(context.Context, model.Artifact, model.FileLink) error
	GetArtifact(context.Context, uuid.UUID) (model.Artifact, error)
	ListArtifacts(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Artifact, string, error)
	SearchFileNodes(context.Context, FileSearchFilter) ([]uuid.UUID, string, error)
	CreateFileLink(context.Context, model.FileLink) error
	GetFileLink(context.Context, uuid.UUID, *uuid.UUID, string) (*model.FileLink, error)
}

type FileSearchFilter struct {
	NamespaceID uuid.UUID
	Query       string
	OwnerID     *uuid.UUID
	Limit       int
	Cursor      string
}
