package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type ListTrash struct {
	NamespaceID uuid.UUID
	Limit       int
	Cursor      string
}

type ListTrashResult struct {
	Items      []repository.TrashEntry
	NextCursor string
}

type trashListReader interface {
	ListTrashEntries(context.Context, repository.TrashListQuery) ([]repository.TrashEntry, string, error)
}

type ListTrashHandler struct {
	repo trashListReader
}

func NewListTrashHandler(repo trashListReader) *ListTrashHandler {
	return &ListTrashHandler{repo: repo}
}

func (h *ListTrashHandler) Handle(ctx context.Context, q ListTrash) (ListTrashResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	items, next, err := h.repo.ListTrashEntries(ctx, repository.TrashListQuery{
		NamespaceID: q.NamespaceID,
		Limit:       limit,
		Cursor:      q.Cursor,
	})
	if err != nil {
		return ListTrashResult{}, err
	}
	return ListTrashResult{Items: items, NextCursor: next}, nil
}
