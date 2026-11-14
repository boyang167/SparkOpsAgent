package query

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type SharedNode = repository.SharedNode

type SharedWithMeFilter = repository.SharedNodeFilter

const (
	SharedCandidateScanBudget = 256
	SharedCandidateChunkSize  = 64
)

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
	authorizer GovernanceAuthorizer
	now        func() time.Time
}

type BatchResourceAuthorizer interface {
	AuthorizeMany(context.Context, resourceauth.BatchRequest) resourceauth.BatchDecisions
}

type GovernanceAuthorizer interface {
	ResourceAuthorizer
	BatchResourceAuthorizer
}

func NewSharedWithMeHandler(repo SharedWithMeRepository, principals resourceauth.PrincipalResolver, authorizer GovernanceAuthorizer) *SharedWithMeHandler {
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
	scanCursor, err := decodeGovernanceCursor(q.Cursor)
	if err != nil {
		return nil, "", err
	}
	scanned := 0
	hasMore := false
	for len(visible) <= limit && scanned < SharedCandidateScanBudget {
		pageStart := scanCursor
		candidates, rawNext, listErr := h.repo.ListSharedNodes(ctx, q.OrganizationID, principals, SharedWithMeFilter{
			NamespaceID: q.NamespaceID, Query: q.Query, Cursor: scanCursor, Limit: SharedCandidateChunkSize, Principals: principals,
		}, h.now())
		if listErr != nil {
			return nil, "", listErr
		}
		if remaining := SharedCandidateScanBudget - scanned; len(candidates) > remaining {
			candidates = candidates[:remaining]
		}
		effective := authorizeSharedCandidates(ctx, h.authorizer, q, candidates)
		for index, candidate := range candidates {
			scanned++
			scanCursor = candidate.Node.ID.String()
			item := effective[index]
			if !containsAction(item.EffectiveActions, model.ResourcePermissionRead) {
				continue
			}
			visible = append(visible, item)
			if len(visible) == limit+1 {
				break
			}
		}
		hasMore = rawNext != ""
		if len(visible) == limit+1 || rawNext == "" || rawNext == pageStart {
			break
		}
		scanCursor = rawNext
	}
	next := ""
	if len(visible) > limit {
		next = encodeGovernanceCursor(visible[limit-1].Node.ID.String())
		visible = visible[:limit]
	} else if scanned == SharedCandidateScanBudget && hasMore {
		next = encodeGovernanceCursor(scanCursor)
	}
	return visible, next, nil
}

func authorizeSharedCandidates(ctx context.Context, authorizer BatchResourceAuthorizer, q SharedWithMe, candidates []SharedNode) []SharedNode {
	results := make([]SharedNode, len(candidates))
	for index, candidate := range candidates {
		results[index].Node = candidate.Node
	}
	if authorizer == nil {
		return results
	}
	decisions := make(resourceauth.BatchDecisions, len(candidates))
	groups := make(map[uuid.UUID][]uuid.UUID)
	for _, candidate := range candidates {
		groups[candidate.Node.NamespaceID] = append(groups[candidate.Node.NamespaceID], candidate.Node.ID)
	}
	for namespaceID, groupNodeIDs := range groups {
		for nodeID, nodeDecisions := range authorizer.AuthorizeMany(ctx, resourceauth.BatchRequest{
			ActorID: q.ActorID, WorkspaceID: q.WorkspaceID, NamespaceID: namespaceID,
			NodeIDs: groupNodeIDs, Permissions: resourceActions,
		}) {
			decisions[nodeID] = nodeDecisions
		}
	}
	for index, candidate := range candidates {
		sourceSet := make(map[uuid.UUID]struct{})
		for _, action := range resourceActions {
			decision := decisions[candidate.Node.ID][action]
			for _, sourceID := range decision.SourceGrantIDs {
				if sourceID != uuid.Nil {
					sourceSet[sourceID] = struct{}{}
				}
			}
			if decision.Allowed {
				results[index].EffectiveActions = append(results[index].EffectiveActions, action)
			}
		}
		for sourceID := range sourceSet {
			results[index].SourceGrantIDs = append(results[index].SourceGrantIDs, sourceID)
		}
		sort.Slice(results[index].SourceGrantIDs, func(i, j int) bool {
			return results[index].SourceGrantIDs[i].String() < results[index].SourceGrantIDs[j].String()
		})
	}
	return results
}

type governanceCursor struct {
	Version int    `json:"v"`
	After   string `json:"after"`
}

func encodeGovernanceCursor(after string) string {
	payload, _ := json.Marshal(governanceCursor{Version: 1, After: after})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeGovernanceCursor(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", domain.ErrInvalidGovernanceCursor
	}
	var cursor governanceCursor
	if json.Unmarshal(payload, &cursor) != nil || cursor.Version != 1 {
		return "", domain.ErrInvalidGovernanceCursor
	}
	id, err := uuid.Parse(cursor.After)
	if err != nil || id == uuid.Nil {
		return "", domain.ErrInvalidGovernanceCursor
	}
	return id.String(), nil
}
