package command

import (
	"context"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type fakeEnvironmentSyncSnapshotRepo struct {
	snapshot *model.EnvironmentSyncSnapshot
}

func (r *fakeEnvironmentSyncSnapshotRepo) Create(context.Context, *model.EnvironmentSyncSnapshot) error {
	return nil
}

func (r *fakeEnvironmentSyncSnapshotRepo) FindByID(context.Context, kernel.EnvironmentSyncSnapshotID) (*model.EnvironmentSyncSnapshot, error) {
	if r.snapshot == nil {
		return nil, domain.ErrEnvironmentSyncSnapshotNotFound
	}
	return r.snapshot, nil
}

func (*fakeEnvironmentSyncSnapshotRepo) BindToRun(context.Context, kernel.EnvironmentSyncSnapshotID, kernel.ProductWorkflowRunID) error {
	return nil
}

type fakeEnvironmentSyncResolver struct {
	target deployport.EnvironmentDeployTarget
}

func (f fakeEnvironmentSyncResolver) Resolve(
	_ context.Context,
	_ kernel.WorkspaceID,
	_ kernel.ProjectID,
	_ kernel.EnvironmentID,
) (deployport.EnvironmentDeployTarget, error) {
	return f.target, nil
}

func TestCreateEnvironmentSyncDeploymentUsesCapturedManifests(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	targetEnvID := kernel.NewEnvironmentID()
	sourceEnvID := kernel.NewEnvironmentID()
	snapshotID := kernel.NewEnvironmentSyncSnapshotID()
	boundRunID := kernel.NewProductWorkflowRunID()
	clusterID := kernel.NewClusterInfoID()

	snapshot := &model.EnvironmentSyncSnapshot{
		ID:                  snapshotID,
		WorkspaceID:         wsID,
		SourceEnvironmentID: sourceEnvID,
		TargetEnvironmentID: targetEnvID,
		BoundRunID:          &boundRunID,
		Projects: []model.EnvironmentSyncProjectSnapshot{{
			ProjectID: projectID,
			Plan: model.EnvironmentSyncPlan{
				Manifests: []model.SyncManifest{{
					Kind: model.SyncManifestKindDeployment,
					Name: "demo",
					YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\n  namespace: target-ns\nspec:\n  template:\n    spec:\n      containers:\n        - name: app\n          image: registry/demo:v2\n",
				}},
				ImageRepository: "registry/demo",
				ImageTag:        "v2",
			},
		}},
	}
	repo := &fakeRunRepo{}
	queue := &recordingQueue{}
	handler := NewCreateEnvironmentSyncDeploymentHandler(
		repo,
		&fakeEnvironmentSyncSnapshotRepo{snapshot: snapshot},
		fakeEnvironmentSyncResolver{target: deployport.EnvironmentDeployTarget{
			EnvironmentID: targetEnvID,
			ClusterID:     clusterID,
			Namespace:     "target-ns",
		}},
		queue,
		time.Now,
	)

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), CreateEnvironmentSyncDeploymentInput{
		SnapshotID: snapshotID,
		ProjectID:  projectID,
	})
	require.NoError(t, err)
	require.NotNil(t, run)
	require.Equal(t, model.RunTypeEnvironmentSync, run.RunType)
	require.False(t, run.CrossCluster)
	require.Equal(t, targetEnvID, *run.EnvironmentID)
	require.Equal(t, sourceEnvID, *run.SourceEnvironmentID)
	require.Equal(t, snapshotID, *run.EnvironmentSyncSnapshotID)
	require.Equal(t, "target-ns", run.DeployNamespace)
	require.Equal(t, "registry/demo", run.ImageRepository)
	require.Equal(t, "v2", run.ImageTag)
	require.Len(t, run.SyncManifests, 1)
	require.Len(t, repo.created, 1)
	require.Len(t, queue.payloads, 1)
}
