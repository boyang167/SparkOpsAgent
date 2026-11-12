package subscriber

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	deploymodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type envProjectRepo struct {
	repository.EnvironmentRepository
	row   *model.EnvironmentProject
	saved *model.EnvironmentProject
}

func (r *envProjectRepo) FindEnvironmentProject(_ context.Context, envID kernel.EnvironmentID, projectID kernel.ProjectID) (*model.EnvironmentProject, error) {
	if r.row == nil {
		return nil, domain.ErrEnvironmentProjectNotFound
	}
	if r.row.EnvironmentID != envID || r.row.ProjectID != projectID {
		return nil, domain.ErrEnvironmentProjectNotFound
	}
	return r.row, nil
}

func (r *envProjectRepo) SaveEnvironmentProject(_ context.Context, row *model.EnvironmentProject) error {
	r.saved = row
	return nil
}

func TestOnDeployCompletedClearsCurrentVersionOnUndeploy(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	projectID := kernel.NewProjectID()
	runID := kernel.NewDeploymentRunID()
	deployedAt := time.Now().UTC()
	repo := &envProjectRepo{row: &model.EnvironmentProject{
		EnvironmentID: envID, ProjectID: projectID, Enabled: true, CurrentImageTag: "v1", CurrentDeploymentRunID: &runID, LastDeployedAt: &deployedAt,
	}}
	ev := deliveryevent.NewDeployCompleted(kernel.NewDeploymentRunID(), projectID, kernel.NewWorkspaceID(), &envID, "", "succeeded", "undeploy", "v1")

	require.NoError(t, NewOnDeployCompleted(repo).Handle(context.Background(), ev))

	require.NotNil(t, repo.saved)
	require.Equal(t, "", repo.saved.CurrentImageTag)
	require.Nil(t, repo.saved.CurrentDeploymentRunID)
}

func TestOnDeployCompletedUpsertsMissingEnvironmentProjectForEnvironmentSync(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	projectID := kernel.NewProjectID()
	runID := kernel.NewDeploymentRunID()
	repo := &envProjectRepo{}
	ev := deliveryevent.NewDeployCompleted(
		runID, projectID, kernel.NewWorkspaceID(), &envID, "", "succeeded",
		string(deploymodel.RunTypeEnvironmentSync), "v2.0.0",
	)

	require.NoError(t, NewOnDeployCompleted(repo).Handle(context.Background(), ev))

	require.NotNil(t, repo.saved)
	require.Equal(t, envID, repo.saved.EnvironmentID)
	require.Equal(t, projectID, repo.saved.ProjectID)
	require.True(t, repo.saved.Enabled)
	require.Equal(t, "v2.0.0", repo.saved.CurrentImageTag)
	require.NotNil(t, repo.saved.CurrentDeploymentRunID)
	require.Equal(t, runID, *repo.saved.CurrentDeploymentRunID)
}

func TestOnDeployCompletedDoesNotOverwriteNewerDeploymentState(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	projectID := kernel.NewProjectID()
	currentRunID := kernel.NewDeploymentRunID()
	deployedAt := time.Now().UTC().Add(time.Minute)
	repo := &envProjectRepo{row: &model.EnvironmentProject{
		EnvironmentID: envID, ProjectID: projectID, Enabled: true, CurrentImageTag: "new",
		CurrentDeploymentRunID: &currentRunID, LastDeployedAt: &deployedAt,
	}}
	ev := deliveryevent.NewDeployCompleted(
		kernel.NewDeploymentRunID(), projectID, kernel.NewWorkspaceID(),
		&envID, "", "succeeded", "normal", "old",
	)

	require.NoError(t, NewOnDeployCompleted(repo).Handle(context.Background(), ev))

	require.Nil(t, repo.saved, "an older event must not overwrite current deployment state")
}
