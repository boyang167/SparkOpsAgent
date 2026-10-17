package subscriber

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type envProjectRepo struct {
	repository.EnvironmentRepository
	row   *model.EnvironmentProject
	saved *model.EnvironmentProject
}

func (r *envProjectRepo) FindEnvironmentProject(context.Context, kernel.EnvironmentID, kernel.ProjectID) (*model.EnvironmentProject, error) {
	return r.row, nil
}

func (r *envProjectRepo) SaveEnvironmentProject(_ context.Context, row *model.EnvironmentProject) error {
	r.saved = row
	return nil
}

func TestOnDeployCompletedClearsCurrentVersionOnUndeploy(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	runID := kernel.NewDeploymentRunID()
	deployedAt := time.Now().UTC()
	repo := &envProjectRepo{row: &model.EnvironmentProject{
		EnvironmentID: envID, Enabled: true, CurrentImageTag: "v1", CurrentDeploymentRunID: &runID, LastDeployedAt: &deployedAt,
	}}
	ev := deliveryevent.NewDeployCompleted(kernel.NewDeploymentRunID(), kernel.NewProjectID(), kernel.NewWorkspaceID(), &envID, "", "succeeded", "undeploy", "v1")

	require.NoError(t, NewOnDeployCompleted(repo).Handle(context.Background(), ev))

	require.NotNil(t, repo.saved)
	require.Equal(t, "", repo.saved.CurrentImageTag)
	require.Nil(t, repo.saved.CurrentDeploymentRunID)
}

func TestOnDeployCompletedDoesNotOverwriteNewerDeploymentState(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	currentRunID := kernel.NewDeploymentRunID()
	deployedAt := time.Now().UTC().Add(time.Minute)
	repo := &envProjectRepo{row: &model.EnvironmentProject{
		EnvironmentID: envID, Enabled: true, CurrentImageTag: "new",
		CurrentDeploymentRunID: &currentRunID, LastDeployedAt: &deployedAt,
	}}
	ev := deliveryevent.NewDeployCompleted(
		kernel.NewDeploymentRunID(), kernel.NewProjectID(), kernel.NewWorkspaceID(),
		&envID, "", "succeeded", "normal", "old",
	)

	require.NoError(t, NewOnDeployCompleted(repo).Handle(context.Background(), ev))

	require.Nil(t, repo.saved, "an older event must not overwrite current deployment state")
}
