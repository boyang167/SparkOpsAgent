package query

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type SharedNode = repository.SharedNode

type SharedWithMeFilter = repository.SharedNodeFilter

type SharedWithMe struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	Query          string
	Cursor         string
	Limit          int
}

type SharedWithMeRepository interface {
	ListSharedNodes(context.Context, uuid.UUID, []repository.PrincipalRef, repository.SharedNodeFilter, time.Time) ([]SharedNode, string, error)
}

type SharedWithMeHandler struct {
	repo       SharedWithMeRepository
	principals resourceauth.PrincipalResolver
	authorizer ResourceAuthorizer
	now        func() time.Time
}

func NewSharedWithMeHandler(repo SharedWithMeRepository, principals resourceauth.PrincipalResolver, authorizer ResourceAuthorizer) *SharedWithMeHandler {
	return &SharedWithMeHandler{repo: repo, principals: principals, authorizer: authorizer, now: func() time.Time { return time.Now().UTC() }}
}

func (h *SharedWithMeHandler) Handle(ctx context.Context, q SharedWithMe) ([]SharedNode, string, error) {
	principals, err := h.principals.Resolve(ctx, resourceauth.ResolvePrincipalsInput{
		ActorID: q.ActorID, OrganizationID: q.OrganizationID, WorkspaceID: q.WorkspaceID,
	})
	if err != nil {
		return nil, "", err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	visible := make([]SharedNode, 0, limit+1)
	scanCursor := q.Cursor
	for len(visible) <= limit {
		pageStart := scanCursor
		candidates, rawNext, listErr := h.repo.ListSharedNodes(ctx, q.OrganizationID, principals, SharedWithMeFilter{
			NamespaceID: q.NamespaceID, Query: q.Query, Cursor: scanCursor, Limit: limit, Principals: principals,
		}, h.now())
		if listErr != nil {
			return nil, "", listErr
		}
		for _, candidate := range candidates {
			scanCursor = candidate.Node.ID.String()
			effective := authorizeSharedCandidate(ctx, h.authorizer, q, candidate.Node)
			if !containsAction(effective.EffectiveActions, model.ResourcePermissionRead) {
				continue
			}
			visible = append(visible, effective)
			if len(visible) == limit+1 {
				break
			}
		}
		if len(visible) == limit+1 || rawNext == "" || rawNext == pageStart {
			break
		}
		scanCursor = rawNext
	}
	next := ""
	if len(visible) > limit {
		next = visible[limit-1].Node.ID.String()
		visible = visible[:limit]
	}
	return visible, next, nil
}

func authorizeSharedCandidate(ctx context.Context, authorizer ResourceAuthorizer, q SharedWithMe, node model.Node) SharedNode {
	result := SharedNode{Node: node}
	sourceSet := make(map[uuid.UUID]struct{})
	if authorizer == nil {
		return result
	}
	for _, action := range resourceActions {
		decision := authorizer.Authorize(ctx, resourceauth.Request{
			ActorID: q.ActorID, WorkspaceID: q.WorkspaceID, NamespaceID: node.NamespaceID,
			NodeID: node.ID, Permission: action,
		})
		for _, sourceID := range decision.SourceGrantIDs {
			if sourceID != uuid.Nil {
				sourceSet[sourceID] = struct{}{}
			}
		}
		if decision.Allowed {
			result.EffectiveActions = append(result.EffectiveActions, action)
		}
	}
	for sourceID := range sourceSet {
		result.SourceGrantIDs = append(result.SourceGrantIDs, sourceID)
	}
	sort.Slice(result.SourceGrantIDs, func(i, j int) bool {
		return result.SourceGrantIDs[i].String() < result.SourceGrantIDs[j].String()
	})
	return result
}
