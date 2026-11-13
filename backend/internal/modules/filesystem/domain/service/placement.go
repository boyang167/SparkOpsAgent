package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

const (
	ruleScopePrefix      = "scope:"
	ruleComplianceEnc    = "compliance:encryption"
	ruleCapabilityPresign = "capability:presigned_upload"
	ruleHealthPrefer     = "health:prefer_healthy"
	ruleFallbackExplicit = "fallback:explicit"
)

type PlacementRequest struct {
	OrganizationID          uuid.UUID
	WorkspaceID             uuid.UUID
	ProjectID               *uuid.UUID
	UserID                  uuid.UUID
	Classification          model.TransferClassification
	Size                    int64
	RequiresPresignedUpload bool
}

type PlacementDecision struct {
	PoolID         uuid.UUID
	ProviderID     uuid.UUID
	PolicyVersion  int64
	MatchedRuleIDs []string
	DecisionReason string
}

type ProviderHealthSnapshot struct {
	ProviderID uuid.UUID
	Status     string
	Latency    time.Duration
}

type PoolSnapshot struct {
	ID            uuid.UUID
	PolicyVersion int64
	Status        model.StoragePoolStatus
	Members       []model.StoragePoolMember
}

type PlacementSnapshot struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	FallbackPoolID uuid.UUID
	Bindings       []model.StorageBinding
	Pools          map[uuid.UUID]PoolSnapshot
	Providers      map[uuid.UUID]model.StorageProvider
	Health         map[uuid.UUID]ProviderHealthSnapshot
}

type Engine struct {
	snapshot PlacementSnapshot
}

func NewPlacementEngine(snapshot PlacementSnapshot) *Engine {
	return &Engine{snapshot: snapshot}
}

func (e *Engine) Place(_ context.Context, req PlacementRequest) (PlacementDecision, error) {
	var empty PlacementDecision
	if req.OrganizationID == uuid.Nil || req.WorkspaceID == uuid.Nil {
		return empty, domain.ErrStoragePlacementUnsatisfied
	}
	poolID, scopeRules, err := e.resolvePool(req)
	if err != nil {
		return empty, err
	}
	pool, ok := e.snapshot.Pools[poolID]
	if !ok || pool.Status != model.StoragePoolActive {
		return empty, domain.ErrStoragePlacementUnsatisfied
	}

	candidates, ruleIDs, pickErr := e.selectPrimaryProvider(req, pool, scopeRules)
	if pickErr != nil {
		return empty, pickErr
	}
	if candidates.providerID == uuid.Nil {
		return empty, domain.ErrStoragePlacementUnsatisfied
	}
	return PlacementDecision{
		PoolID:         poolID,
		ProviderID:     candidates.providerID,
		PolicyVersion:  pool.PolicyVersion,
		MatchedRuleIDs: appendUnique(ruleIDs, candidates.ruleIDs...),
		DecisionReason: candidates.reason,
	}, nil
}

type primaryPick struct {
	providerID uuid.UUID
	ruleIDs    []string
	reason     string
}

type viablePrimary struct {
	member model.StoragePoolMember
	health ProviderHealthSnapshot
}

func (e *Engine) resolvePool(req PlacementRequest) (uuid.UUID, []string, error) {
	if binding, rules, ok := e.bestScopeBinding(req); ok {
		return binding.PoolID, rules, nil
	}
	if e.snapshot.FallbackPoolID != uuid.Nil {
		return e.snapshot.FallbackPoolID, []string{ruleFallbackExplicit}, nil
	}
	return uuid.Nil, nil, domain.ErrStoragePlacementUnsatisfied
}

func (e *Engine) bestScopeBinding(req PlacementRequest) (model.StorageBinding, []string, bool) {
	type candidate struct {
		binding model.StorageBinding
		rank    int
	}
	var matches []candidate
	for _, binding := range e.snapshot.Bindings {
		if binding.OrganizationID != req.OrganizationID {
			continue
		}
		if !scopeMatches(binding, req) {
			continue
		}
		matches = append(matches, candidate{
			binding: binding,
			rank:    scopeSpecificity(binding.ScopeType),
		})
	}
	if len(matches) == 0 {
		return model.StorageBinding{}, nil, false
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].rank != matches[j].rank {
			return matches[i].rank > matches[j].rank
		}
		if matches[i].binding.Priority != matches[j].binding.Priority {
			return matches[i].binding.Priority > matches[j].binding.Priority
		}
		return matches[i].binding.PoolID.String() < matches[j].binding.PoolID.String()
	})
	selected := matches[0].binding
	return selected, []string{ruleScopePrefix + string(selected.ScopeType)}, true
}

func scopeMatches(binding model.StorageBinding, req PlacementRequest) bool {
	switch binding.ScopeType {
	case model.ScopeOrganization:
		return binding.ScopeID == req.OrganizationID
	case model.ScopeWorkspace:
		return binding.ScopeID == req.WorkspaceID
	case model.ScopeProject:
		return req.ProjectID != nil && binding.ScopeID == *req.ProjectID
	case model.ScopeUser:
		return binding.ScopeID == req.UserID
	default:
		return false
	}
}

func scopeSpecificity(scope model.ScopeType) int {
	switch scope {
	case model.ScopeUser:
		return 4
	case model.ScopeProject:
		return 3
	case model.ScopeWorkspace:
		return 2
	case model.ScopeOrganization:
		return 1
	default:
		return 0
	}
}

func (e *Engine) selectPrimaryProvider(req PlacementRequest, pool PoolSnapshot, priorRules []string) (primaryPick, []string, error) {
	members := append([]model.StoragePoolMember(nil), pool.Members...)
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].Priority != members[j].Priority {
			return members[i].Priority < members[j].Priority
		}
		return members[i].ProviderID.String() < members[j].ProviderID.String()
	})

	requiresEncryption := req.Classification == model.TransferClassificationRestricted ||
		req.Classification == model.TransferClassificationConfidential

	var viablePrimaries []viablePrimary
	ruleIDs := append([]string(nil), priorRules...)

	for _, member := range members {
		if member.Mode != model.ProviderModePrimary {
			continue
		}
		provider, ok := e.snapshot.Providers[member.ProviderID]
		if !ok || provider.Status != "" && provider.Status != "ACTIVE" {
			continue
		}
		if requiresEncryption && !storage.HasEncryptionCapability(provider.Capabilities) {
			continue
		}
		if req.RequiresPresignedUpload && !capabilityEnabled(provider.Capabilities, "presigned_upload") {
			continue
		}
		if maxSize := capabilityInt64(provider.Capabilities, "max_object_bytes"); maxSize > 0 && req.Size > maxSize {
			continue
		}
		health := e.snapshot.Health[member.ProviderID]
		viablePrimaries = append(viablePrimaries, viablePrimary{member: member, health: health})
	}

	if requiresEncryption && len(viablePrimaries) == 0 {
		hasPrimary := false
		for _, member := range members {
			if member.Mode == model.ProviderModePrimary {
				hasPrimary = true
				break
			}
		}
		if hasPrimary {
			return primaryPick{}, ruleIDs, domain.ErrStorageComplianceUnsatisfied
		}
	}
	if len(viablePrimaries) == 0 {
		return primaryPick{}, ruleIDs, domain.ErrStoragePlacementUnsatisfied
	}

	if requiresEncryption {
		ruleIDs = appendUnique(ruleIDs, ruleComplianceEnc)
	}
	if req.RequiresPresignedUpload {
		ruleIDs = appendUnique(ruleIDs, ruleCapabilityPresign)
	}

	healthy := filterHealthy(viablePrimaries)
	if len(healthy) > 0 && len(healthy) < len(viablePrimaries) {
		ruleIDs = appendUnique(ruleIDs, ruleHealthPrefer)
		viablePrimaries = healthy
	}
	sort.SliceStable(viablePrimaries, func(i, j int) bool {
		if healthRank(viablePrimaries[i].health) != healthRank(viablePrimaries[j].health) {
			return healthRank(viablePrimaries[i].health) < healthRank(viablePrimaries[j].health)
		}
		if viablePrimaries[i].health.Latency != viablePrimaries[j].health.Latency {
			return viablePrimaries[i].health.Latency < viablePrimaries[j].health.Latency
		}
		return viablePrimaries[i].member.ProviderID.String() < viablePrimaries[j].member.ProviderID.String()
	})

	selected := viablePrimaries[0]
	return primaryPick{
		providerID: selected.member.ProviderID,
		reason:     fmt.Sprintf("pool:%s:primary:%s", pool.ID, selected.member.ProviderID),
	}, ruleIDs, nil
}

func filterHealthy(items []viablePrimary) []viablePrimary {
	out := make([]viablePrimary, 0, len(items))
	for _, item := range items {
		if item.health.Status == "" || item.health.Status == "HEALTHY" {
			out = append(out, item)
		}
	}
	return out
}

func healthRank(health ProviderHealthSnapshot) int {
	if health.Status == "" || health.Status == "HEALTHY" {
		return 0
	}
	return 1
}

func capabilityEnabled(capabilities map[string]any, key string) bool {
	if capabilities == nil {
		return false
	}
	value, ok := capabilities[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "TRUE"
	default:
		return false
	}
}

func capabilityInt64(capabilities map[string]any, key string) int64 {
	if capabilities == nil {
		return 0
	}
	value, ok := capabilities[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func appendUnique(base []string, extra ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, item := range append(base, extra...) {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
