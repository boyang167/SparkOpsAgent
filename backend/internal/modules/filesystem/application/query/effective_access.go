package query

import (
	"context"
	"sort"

	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type EffectiveAccess struct {
	ActorID     uuid.UUID
	WorkspaceID uuid.UUID
	NamespaceID uuid.UUID
	NodeID      uuid.UUID
}

type EffectiveAccessResult struct {
	Decision         string
	EffectiveActions []model.ResourcePermission
	ReasonCode       string
	SourceGrantIDs   []uuid.UUID
	PolicyVersion    int64
	CacheKey         resourceauth.AuthorizationCacheKey
}

type EffectiveAccessHandler struct {
	authorizer ResourceAuthorizer
}

func NewEffectiveAccessHandler(authorizer ResourceAuthorizer) *EffectiveAccessHandler {
	return &EffectiveAccessHandler{authorizer: authorizer}
}

func (h *EffectiveAccessHandler) Handle(ctx context.Context, q EffectiveAccess) EffectiveAccessResult {
	result := EffectiveAccessResult{Decision: "DENY"}
	sourceIDs := make(map[uuid.UUID]struct{})
	denialReason := ""
	for _, action := range resourceActions {
		decision := h.authorizer.Authorize(ctx, resourceauth.Request{
			ActorID: q.ActorID, WorkspaceID: q.WorkspaceID, NamespaceID: q.NamespaceID,
			NodeID: q.NodeID, Permission: action,
		})
		if result.PolicyVersion == 0 {
			result.PolicyVersion = decision.PolicyVersion
			result.CacheKey = decision.CacheKey
			result.ReasonCode = decision.Reason
		}
		for _, id := range decision.SourceGrantIDs {
			sourceIDs[id] = struct{}{}
		}
		if !decision.Allowed {
			if decision.Reason == "explicit_deny" || decision.Reason == "admin_deny" {
				denialReason = decision.Reason
			}
			continue
		}
		if result.Decision == "DENY" {
			result.Decision = "ALLOW"
			result.ReasonCode = decision.Reason
			result.CacheKey = decision.CacheKey
		}
		result.EffectiveActions = append(result.EffectiveActions, action)
	}
	if result.Decision == "DENY" && denialReason != "" {
		result.ReasonCode = denialReason
	}
	for id := range sourceIDs {
		result.SourceGrantIDs = append(result.SourceGrantIDs, id)
	}
	sort.Slice(result.SourceGrantIDs, func(i, j int) bool {
		return result.SourceGrantIDs[i].String() < result.SourceGrantIDs[j].String()
	})
	return result
}
