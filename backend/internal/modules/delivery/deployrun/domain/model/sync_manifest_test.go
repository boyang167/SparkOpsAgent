package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/stretchr/testify/require"
)

func TestValidateSyncManifestsRejectsEmpty(t *testing.T) {
	err := model.ValidateSyncManifests(nil, "demo")
	require.ErrorIs(t, err, domain.ErrDeploymentSyncManifestsEmpty)
}

func TestValidateSyncManifestsAcceptsManagedPullSecret(t *testing.T) {
	err := model.ValidateSyncManifests([]model.SyncManifest{{
		Kind: model.SyncManifestKindSecret, Name: "registry-demo", ManagedPullSecret: true,
	}}, "demo")
	require.NoError(t, err)
}

func TestValidateSyncManifestsRejectsKindMismatch(t *testing.T) {
	err := model.ValidateSyncManifests([]model.SyncManifest{{
		Kind: model.SyncManifestKindConfigMap,
		Name: "app-config",
		YAML: "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app-config\n",
	}}, "demo")
	require.ErrorIs(t, err, domain.ErrDeploymentSyncManifestInvalid)
}

func TestValidateSyncManifestsRejectsNameChange(t *testing.T) {
	err := model.ValidateSyncManifests([]model.SyncManifest{{
		Kind: model.SyncManifestKindConfigMap,
		Name: "app-config",
		YAML: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: other\n",
	}}, "demo")
	require.ErrorIs(t, err, domain.ErrDeploymentSyncManifestInvalid)
}

func TestSyncApplyOrder(t *testing.T) {
	ordered := model.SyncApplyOrder([]model.SyncManifest{
		{Kind: model.SyncManifestKindDeployment, Name: "app"},
		{Kind: model.SyncManifestKindConfigMap, Name: "cfg"},
		{Kind: model.SyncManifestKindService, Name: "svc"},
	})
	require.Equal(t, model.SyncManifestKindConfigMap, ordered[0].Kind)
	require.Equal(t, model.SyncManifestKindService, ordered[1].Kind)
	require.Equal(t, model.SyncManifestKindDeployment, ordered[2].Kind)
}
