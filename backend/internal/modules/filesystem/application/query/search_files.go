package query

import (
	"context"
	"strings"

	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type SearchFiles struct {
	ActorID       uuid.UUID
	WorkspaceID   uuid.UUID
	NamespaceID   uuid.UUID
	Query         string
	OwnerID       *uuid.UUID
	Limit         int
	Cursor        string
	FullText      bool
}

type SearchHit struct {
	NodeID uuid.UUID
}

type FileSearchIndex interface {
	Search(context.Context, SearchFiles) ([]uuid.UUID, string, error)
}

type SearchAuthorizer interface {
	Allow(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) bool
}

type RepositorySearchIndex struct {
	repo repository.FilesystemRepository
}

func NewRepositorySearchIndex(repo repository.FilesystemRepository) *RepositorySearchIndex {
	return &RepositorySearchIndex{repo: repo}
}

func (i *RepositorySearchIndex) Search(ctx context.Context, q SearchFiles) ([]uuid.UUID, string, error) {
	if i == nil || i.repo == nil {
		return nil, "", nil
	}
	return i.repo.SearchFileNodes(ctx, repository.FileSearchFilter{
		NamespaceID: q.NamespaceID,
		Query:       q.Query,
		OwnerID:     q.OwnerID,
		Limit:       q.Limit,
		Cursor:      q.Cursor,
	})
}

type AuthorizationSearchAuthorizer struct {
	svc *resourceauth.Service
}

func NewAuthorizationSearchAuthorizer(svc *resourceauth.Service) AuthorizationSearchAuthorizer {
	return AuthorizationSearchAuthorizer{svc: svc}
}

func (a AuthorizationSearchAuthorizer) Allow(ctx context.Context, actorID, workspaceID, namespaceID, nodeID uuid.UUID) bool {
	if a.svc == nil {
		return false
	}
	decision := a.svc.Authorize(ctx, resourceauth.Request{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespaceID,
		NodeID: nodeID, Permission: model.ResourcePermissionRead,
	})
	return decision.Allowed
}

type SearchFilesHandler struct {
	index    FileSearchIndex
	fullText FileSearchIndex
	authorizer SearchAuthorizer
}

func NewSearchFilesHandler(index FileSearchIndex, fullText FileSearchIndex, authorizer SearchAuthorizer) *SearchFilesHandler {
	return &SearchFilesHandler{index: index, fullText: fullText, authorizer: authorizer}
}

func (h *SearchFilesHandler) Handle(ctx context.Context, q SearchFiles) ([]SearchHit, string, error) {
	if h == nil || h.index == nil {
		return nil, "", nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	q.Limit = limit

	candidates, nextCursor, err := h.index.Search(ctx, q)
	if err != nil {
		return nil, "", err
	}
	if q.FullText && h.fullText != nil && strings.TrimSpace(q.Query) != "" {
		ftCandidates, ftNext, ftErr := h.fullText.Search(ctx, q)
		if ftErr == nil {
			seen := make(map[uuid.UUID]struct{}, len(candidates)+len(ftCandidates))
			merged := make([]uuid.UUID, 0, len(candidates)+len(ftCandidates))
			for _, id := range candidates {
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				merged = append(merged, id)
			}
			for _, id := range ftCandidates {
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				merged = append(merged, id)
			}
			candidates = merged
			if ftNext != "" {
				nextCursor = ftNext
			}
		}
	}

	allowed := make([]SearchHit, 0, len(candidates))
	for _, nodeID := range candidates {
		if h.authorizer != nil && !h.authorizer.Allow(ctx, q.ActorID, q.WorkspaceID, q.NamespaceID, nodeID) {
			continue
		}
		allowed = append(allowed, SearchHit{NodeID: nodeID})
	}
	return allowed, nextCursor, nil
}
