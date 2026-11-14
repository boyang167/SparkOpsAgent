package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func TestPlacementOrdersScopeComplianceCapabilityHealthFallback(t *testing.T) {
	orgID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()

	localID := uuid.New()
	restrictedS3ID := uuid.New()
	orgPoolID := uuid.New()
	restrictedS3PoolID := uuid.New()
	policyVersion := int64(7)

	snapshot := PlacementSnapshot{
		Bindings: []model.StorageBinding{
			{ID: uuid.New(), ScopeType: model.ScopeOrganization, ScopeID: orgID, PoolID: orgPoolID, Priority: 1, OrganizationID: orgID},
			{ID: uuid.New(), ScopeType: model.ScopeWorkspace, ScopeID: workspaceID, PoolID: restrictedS3PoolID, Priority: 1, OrganizationID: orgID},
		},
		Pools: map[uuid.UUID]PoolSnapshot{
			orgPoolID: {
				ID: orgPoolID, PolicyVersion: 1, Status: model.StoragePoolActive,
				Members: []model.StoragePoolMember{{ProviderID: localID, Mode: model.ProviderModePrimary, Priority: 10}},
			},
			restrictedS3PoolID: {
				ID: restrictedS3PoolID, PolicyVersion: policyVersion, Status: model.StoragePoolActive,
				Members: []model.StoragePoolMember{{ProviderID: restrictedS3ID, Mode: model.ProviderModePrimary, Priority: 10}},
			},
		},
		Providers: map[uuid.UUID]model.StorageProvider{
			localID: mustProvider(t, localID, orgID, model.ProviderTypeLocal, map[string]any{"presigned_upload": false}),
			restrictedS3ID: mustProvider(t, restrictedS3ID, orgID, model.ProviderTypeS3, map[string]any{
				"encryption": true, "presigned_upload": true,
			}),
		},
		Health: map[uuid.UUID]ProviderHealthSnapshot{
			localID:        {ProviderID: localID, Status: "HEALTHY", Latency: time.Millisecond},
			restrictedS3ID: {ProviderID: restrictedS3ID, Status: "HEALTHY", Latency: 2 * time.Millisecond},
		},
	}
	engine := NewPlacementEngine(snapshot)
	decision, err := engine.Place(context.Background(), PlacementRequest{
		OrganizationID:          orgID,
		WorkspaceID:             workspaceID,
		UserID:                  userID,
		Classification:          model.TransferClassificationRestricted,
		Size:                    1024,
		RequiresPresignedUpload: true,
	})
	require.NoError(t, err)
	require.Equal(t, restrictedS3PoolID, decision.PoolID)
	require.Equal(t, restrictedS3ID, decision.ProviderID)
	require.Equal(t, policyVersion, decision.PolicyVersion)
	require.Contains(t, decision.MatchedRuleIDs, "scope:WORKSPACE")
	require.Contains(t, decision.MatchedRuleIDs, "compliance:encryption")
	require.Contains(t, decision.MatchedRuleIDs, "capability:presigned_upload")
}

func TestPlacementRejectsRestrictedWithoutEncryption(t *testing.T) {
	orgID := uuid.New()
	workspaceID := uuid.New()
	poolID := uuid.New()
	localID := uuid.New()
	snapshot := PlacementSnapshot{
		Bindings: []model.StorageBinding{
			{ID: uuid.New(), ScopeType: model.ScopeWorkspace, ScopeID: workspaceID, PoolID: poolID, Priority: 1, OrganizationID: orgID},
		},
		Pools: map[uuid.UUID]PoolSnapshot{
			poolID: {
				ID: poolID, Status: model.StoragePoolActive,
				Members: []model.StoragePoolMember{{ProviderID: localID, Mode: model.ProviderModePrimary, Priority: 10}},
			},
		},
		Providers: map[uuid.UUID]model.StorageProvider{
			localID: mustProvider(t, localID, orgID, model.ProviderTypeLocal, map[string]any{"presigned_upload": false}),
		},
		Health: map[uuid.UUID]ProviderHealthSnapshot{
			localID: {ProviderID: localID, Status: "HEALTHY"},
		},
	}
	engine := NewPlacementEngine(snapshot)
	_, err := engine.Place(context.Background(), PlacementRequest{
		OrganizationID: orgID, WorkspaceID: workspaceID, UserID: uuid.New(),
		Classification: model.TransferClassificationConfidential, Size: 512,
	})
	require.ErrorIs(t, err, domain.ErrStorageComplianceUnsatisfied)
}

func TestPlacementDeterministicForIdenticalInput(t *testing.T) {
	snapshot := restrictedWorkspaceSnapshot(t)
	engine := NewPlacementEngine(snapshot)
	req := PlacementRequest{
		OrganizationID:          snapshot.OrganizationID,
		WorkspaceID:             snapshot.WorkspaceID,
		UserID:                  uuid.New(),
		Classification:          model.TransferClassificationRestricted,
		Size:                    2048,
		RequiresPresignedUpload: true,
	}
	first, err := engine.Place(context.Background(), req)
	require.NoError(t, err)
	second, err := engine.Place(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func TestPlacementPrefersHealthyPrimaryOverUnhealthy(t *testing.T) {
	orgID := uuid.New()
	workspaceID := uuid.New()
	poolID := uuid.New()
	healthyID := uuid.New()
	sickID := uuid.New()
	snapshot := PlacementSnapshot{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Bindings: []model.StorageBinding{
			{ID: uuid.New(), ScopeType: model.ScopeWorkspace, ScopeID: workspaceID, PoolID: poolID, Priority: 1, OrganizationID: orgID},
		},
		Pools: map[uuid.UUID]PoolSnapshot{
			poolID: {
				ID: poolID, Status: model.StoragePoolActive,
				Members: []model.StoragePoolMember{
					{ProviderID: sickID, Mode: model.ProviderModePrimary, Priority: 5},
					{ProviderID: healthyID, Mode: model.ProviderModePrimary, Priority: 10},
				},
			},
		},
		Providers: map[uuid.UUID]model.StorageProvider{
			healthyID: mustProvider(t, healthyID, orgID, model.ProviderTypeS3, map[string]any{"encryption": true, "presigned_upload": true}),
			sickID:    mustProvider(t, sickID, orgID, model.ProviderTypeS3, map[string]any{"encryption": true, "presigned_upload": true}),
		},
		Health: map[uuid.UUID]ProviderHealthSnapshot{
			healthyID: {ProviderID: healthyID, Status: "HEALTHY", Latency: time.Millisecond},
			sickID:    {ProviderID: sickID, Status: "UNHEALTHY", Latency: time.Millisecond},
		},
	}
	engine := NewPlacementEngine(snapshot)
	decision, err := engine.Place(context.Background(), PlacementRequest{
		OrganizationID: orgID, WorkspaceID: workspaceID, UserID: uuid.New(),
		Classification: model.TransferClassificationRestricted, Size: 1024, RequiresPresignedUpload: true,
	})
	require.NoError(t, err)
	require.Equal(t, healthyID, decision.ProviderID)
	require.Contains(t, decision.MatchedRuleIDs, "health:prefer_healthy")
}

func TestPlacementUsesExplicitFallbackPoolWhenNoBinding(t *testing.T) {
	orgID := uuid.New()
	workspaceID := uuid.New()
	fallbackPoolID := uuid.New()
	providerID := uuid.New()
	snapshot := PlacementSnapshot{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		FallbackPoolID: fallbackPoolID,
		Pools: map[uuid.UUID]PoolSnapshot{
			fallbackPoolID: {
				ID: fallbackPoolID, PolicyVersion: 2, Status: model.StoragePoolActive,
				Members: []model.StoragePoolMember{{ProviderID: providerID, Mode: model.ProviderModePrimary, Priority: 10}},
			},
		},
		Providers: map[uuid.UUID]model.StorageProvider{
			providerID: mustProvider(t, providerID, orgID, model.ProviderTypeLocal, map[string]any{}),
		},
		Health: map[uuid.UUID]ProviderHealthSnapshot{
			providerID: {ProviderID: providerID, Status: "HEALTHY"},
		},
	}
	engine := NewPlacementEngine(snapshot)
	decision, err := engine.Place(context.Background(), PlacementRequest{
		OrganizationID: orgID, WorkspaceID: workspaceID, UserID: uuid.New(),
		Classification: model.TransferClassificationInternal, Size: 100,
	})
	require.NoError(t, err)
	require.Equal(t, fallbackPoolID, decision.PoolID)
	require.Equal(t, providerID, decision.ProviderID)
	require.Contains(t, decision.MatchedRuleIDs, "fallback:explicit")
}

func TestPlacementNamespaceScopePrecedence(t *testing.T) {
	orgID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()
	orgPoolID := uuid.New()
	workspacePoolID := uuid.New()
	userPoolID := uuid.New()
	systemPoolID := uuid.New()
	pools := map[uuid.UUID]PoolSnapshot{}
	providers := map[uuid.UUID]model.StorageProvider{}
	for _, poolID := range []uuid.UUID{orgPoolID, workspacePoolID, userPoolID, systemPoolID} {
		providerID := uuid.New()
		pools[poolID] = PoolSnapshot{
			ID: poolID, Status: model.StoragePoolActive,
			Members: []model.StoragePoolMember{{ProviderID: providerID, Mode: model.ProviderModePrimary}},
		}
		providers[providerID] = mustProvider(t, providerID, orgID, model.ProviderTypeS3, map[string]any{})
	}
	snapshot := PlacementSnapshot{
		OrganizationID: orgID,
		FallbackPoolID: systemPoolID,
		Bindings: []model.StorageBinding{
			{ID: uuid.New(), OrganizationID: orgID, ScopeType: model.ScopeOrganization, ScopeID: orgID, PoolID: orgPoolID},
			{ID: uuid.New(), OrganizationID: orgID, ScopeType: model.ScopeWorkspace, ScopeID: workspaceID, PoolID: workspacePoolID},
			{ID: uuid.New(), OrganizationID: orgID, ScopeType: model.ScopeUser, ScopeID: userID, PoolID: userPoolID},
		},
		Pools: pools, Providers: providers,
	}
	engine := NewPlacementEngine(snapshot)

	tests := []struct {
		name          string
		namespaceType model.NamespaceType
		workspaceID   uuid.UUID
		userID        uuid.UUID
		wantPoolID    uuid.UUID
		wantRule      string
	}{
		{"user uses user binding", model.NamespaceTypeUser, uuid.Nil, userID, userPoolID, "scope:USER"},
		{"workspace uses workspace binding", model.NamespaceTypeWorkspace, workspaceID, uuid.Nil, workspacePoolID, "scope:WORKSPACE"},
		{"organization uses organization binding", model.NamespaceTypeOrganization, uuid.Nil, uuid.Nil, orgPoolID, "scope:ORGANIZATION"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := engine.Place(context.Background(), PlacementRequest{
				OrganizationID: orgID, NamespaceType: tt.namespaceType,
				WorkspaceID: tt.workspaceID, UserID: tt.userID,
				Classification: model.TransferClassificationInternal,
			})
			require.NoError(t, err)
			require.Equal(t, tt.wantPoolID, decision.PoolID)
			require.Contains(t, decision.MatchedRuleIDs, tt.wantRule)
		})
	}
}

func TestPlacementSkipsDisabledAndWrongOrganizationBindingsThenFallsBack(t *testing.T) {
	orgID := uuid.New()
	workspaceID := uuid.New()
	disabledPoolID := uuid.New()
	orgPoolID := uuid.New()
	systemPoolID := uuid.New()
	wrongPoolID := uuid.New()
	pools := map[uuid.UUID]PoolSnapshot{}
	providers := map[uuid.UUID]model.StorageProvider{}
	for poolID, status := range map[uuid.UUID]model.StoragePoolStatus{
		disabledPoolID: model.StoragePoolDisabled,
		orgPoolID:      model.StoragePoolActive,
		systemPoolID:   model.StoragePoolActive,
		wrongPoolID:    model.StoragePoolActive,
	} {
		providerID := uuid.New()
		pools[poolID] = PoolSnapshot{
			ID: poolID, Status: status,
			Members: []model.StoragePoolMember{{ProviderID: providerID, Mode: model.ProviderModePrimary}},
		}
		providers[providerID] = mustProvider(t, providerID, orgID, model.ProviderTypeS3, map[string]any{})
	}
	engine := NewPlacementEngine(PlacementSnapshot{
		OrganizationID: orgID, FallbackPoolID: systemPoolID,
		Bindings: []model.StorageBinding{
			{ID: uuid.New(), OrganizationID: orgID, ScopeType: model.ScopeWorkspace, ScopeID: workspaceID, PoolID: disabledPoolID},
			{ID: uuid.New(), OrganizationID: uuid.New(), ScopeType: model.ScopeWorkspace, ScopeID: workspaceID, PoolID: wrongPoolID},
			{ID: uuid.New(), OrganizationID: orgID, ScopeType: model.ScopeOrganization, ScopeID: orgID, PoolID: orgPoolID},
		},
		Pools: pools, Providers: providers,
	})
	decision, err := engine.Place(context.Background(), PlacementRequest{
		OrganizationID: orgID, NamespaceType: model.NamespaceTypeWorkspace,
		WorkspaceID: workspaceID, Classification: model.TransferClassificationInternal,
	})
	require.NoError(t, err)
	require.Equal(t, orgPoolID, decision.PoolID)
	require.Contains(t, decision.MatchedRuleIDs, "scope:ORGANIZATION")

	engine = NewPlacementEngine(PlacementSnapshot{
		OrganizationID: orgID, FallbackPoolID: systemPoolID,
		Bindings: []model.StorageBinding{
			{ID: uuid.New(), OrganizationID: orgID, ScopeType: model.ScopeWorkspace, ScopeID: workspaceID, PoolID: disabledPoolID},
		},
		Pools: pools, Providers: providers,
	})
	decision, err = engine.Place(context.Background(), PlacementRequest{
		OrganizationID: orgID, NamespaceType: model.NamespaceTypeWorkspace,
		WorkspaceID: workspaceID, Classification: model.TransferClassificationInternal,
	})
	require.NoError(t, err)
	require.Equal(t, systemPoolID, decision.PoolID)
	require.Contains(t, decision.MatchedRuleIDs, "fallback:explicit")
}

func mustProvider(t *testing.T, id, orgID uuid.UUID, providerType model.ProviderType, caps map[string]any) model.StorageProvider {
	t.Helper()
	provider, err := model.NewStorageProvider(model.StorageProviderInput{
		ID: id, OrganizationID: orgID, Name: id.String(), ProviderType: providerType,
		Endpoint: "endpoint", SecretRef: "vault://test", Capabilities: caps,
	})
	require.NoError(t, err)
	return *provider
}

func restrictedWorkspaceSnapshot(t *testing.T) PlacementSnapshot {
	t.Helper()
	orgID := uuid.New()
	workspaceID := uuid.New()
	poolID := uuid.New()
	providerID := uuid.New()
	return PlacementSnapshot{
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Bindings: []model.StorageBinding{
			{ID: uuid.New(), ScopeType: model.ScopeWorkspace, ScopeID: workspaceID, PoolID: poolID, Priority: 1, OrganizationID: orgID},
		},
		Pools: map[uuid.UUID]PoolSnapshot{
			poolID: {
				ID: poolID, PolicyVersion: 9, Status: model.StoragePoolActive,
				Members: []model.StoragePoolMember{{ProviderID: providerID, Mode: model.ProviderModePrimary, Priority: 10}},
			},
		},
		Providers: map[uuid.UUID]model.StorageProvider{
			providerID: mustProvider(t, providerID, orgID, model.ProviderTypeS3, map[string]any{"encryption": true, "presigned_upload": true}),
		},
		Health: map[uuid.UUID]ProviderHealthSnapshot{
			providerID: {ProviderID: providerID, Status: "HEALTHY", Latency: time.Millisecond},
		},
	}
}
