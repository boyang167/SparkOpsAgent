package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

var (
	ErrNodeNameConflict = domain.ErrNodeNameConflict
	ErrNodeNotFound     = domain.ErrNodeNotFound
)

type FilesystemRepository interface {
	CreateNamespace(context.Context, *model.Namespace) error
	GetNamespaceByWorkspace(context.Context, uuid.UUID) (*model.Namespace, error)
	SetNamespaceRoot(context.Context, uuid.UUID, uuid.UUID) error
	CreateNode(context.Context, *model.Node) error
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	ListChildren(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error)
	SaveRevision(context.Context, *model.FileRevision, *model.Blob, *model.StorageReplica) error
	GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error)
	ListRevisions(context.Context, uuid.UUID, uuid.UUID, int, string) ([]model.FileRevision, string, error)
	SetCurrentRevision(context.Context, uuid.UUID, uuid.UUID) error
	IncrementBlobRef(context.Context, uuid.UUID) error
	GetBlob(context.Context, uuid.UUID) (*model.Blob, error)
	GetAvailableReplica(context.Context, uuid.UUID, uuid.UUID) (*model.StorageReplica, error)
	ApplyQuotaUsage(context.Context, uuid.UUID, int64, int64) error
	RenameNode(context.Context, uuid.UUID, string) error
	MoveNode(context.Context, uuid.UUID, *uuid.UUID, string) error
	TrashNode(context.Context, uuid.UUID) error
}
