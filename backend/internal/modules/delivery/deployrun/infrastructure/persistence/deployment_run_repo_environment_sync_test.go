package persistence

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestDeploymentRunRepositoryPersistsEnvironmentSyncProvenance(t *testing.T) {
	repo, _ := newRunRepo(t)
	sourceEnvID := kernel.NewEnvironmentID()
	snapshotID := kernel.NewEnvironmentSyncSnapshotID()
	run := model.NewEnvironmentSyncDeploymentRun(
		kernel.NewWorkspaceID(),
		kernel.NewProjectID(),
		kernel.NewClusterInfoID(),
		kernel.NewMemberID(),
	)
	targetEnvID := kernel.NewEnvironmentID()
	run.EnvironmentID = &targetEnvID
	run.SourceEnvironmentID = &sourceEnvID
	run.EnvironmentSyncSnapshotID = &snapshotID
	run.DeployNamespace = "target-ns"
	run.SyncManifests = []model.SyncManifest{{
		Kind: model.SyncManifestKindDeployment,
		Name: "demo",
		YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\n",
	}}

	require.NoError(t, repo.Create(context.Background(), run))

	loaded, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.RunTypeEnvironmentSync, loaded.RunType)
	require.Equal(t, sourceEnvID, *loaded.SourceEnvironmentID)
	require.Equal(t, snapshotID, *loaded.EnvironmentSyncSnapshotID)
	require.Equal(t, targetEnvID, *loaded.EnvironmentID)
	require.Len(t, loaded.SyncManifests, 1)
}
