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
	CreateNode(context.Context, *model.Node) error
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	ListChildren(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error)
	SaveRevision(context.Context, *model.FileRevision, *model.Blob, *model.StorageReplica) error
	SetCurrentRevision(context.Context, uuid.UUID, uuid.UUID) error
	MoveNode(context.Context, uuid.UUID, *uuid.UUID, string) error
	TrashNode(context.Context, uuid.UUID) error
}
