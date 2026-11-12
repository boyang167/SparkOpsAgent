package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type ListChildren struct {
	NamespaceID uuid.UUID
	ParentID    *uuid.UUID
	Limit       int
	Cursor      string
}

type ListChildrenHandler struct {
	repo repository.FilesystemRepository
}

func NewListChildrenHandler(repo repository.FilesystemRepository) *ListChildrenHandler {
	return &ListChildrenHandler{repo: repo}
}

func (h *ListChildrenHandler) Handle(ctx context.Context, q ListChildren) ([]model.Node, string, error) {
	return h.repo.ListChildren(ctx, q.NamespaceID, q.ParentID, q.Limit, q.Cursor)
}

type ListRevisions struct {
	NamespaceID uuid.UUID
	NodeID      uuid.UUID
	Limit       int
	Cursor      string
}

type ListRevisionsHandler struct {
	repo repository.FilesystemRepository
}

func NewListRevisionsHandler(repo repository.FilesystemRepository) *ListRevisionsHandler {
	return &ListRevisionsHandler{repo: repo}
}

func (h *ListRevisionsHandler) Handle(ctx context.Context, q ListRevisions) ([]model.FileRevision, string, error) {
	return h.repo.ListRevisions(ctx, q.NodeID, q.NamespaceID, q.Limit, q.Cursor)
}
