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
		WorkspaceID:  workspaceID,
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
		WorkspaceID:  workspaceID,
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
		WorkspaceID:  workspaceID,
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
