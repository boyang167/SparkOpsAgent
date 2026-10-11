package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

type recordingRemover struct {
	calls  int
	target runtimeport.DeploymentTarget
	spec   runtimeport.RemoveSpec
	err    error
}

func (r *recordingRemover) Remove(_ context.Context, target runtimeport.DeploymentTarget, spec runtimeport.RemoveSpec) error {
	r.calls++
	r.target = target
	r.spec = spec
	return r.err
}

func undeployFixture(t *testing.T, status model.Status) (*orchestratorFixture, *recordingRemover) {
	t.Helper()
	f := newOrchestratorFixture(status)
	baseline := *f.run
	baseline.DeployNamespace = "team-a-dev"
	undeploy := model.NewUndeployRun(f.wsID, f.run.ProjectID, f.run.CreatedBy, &baseline, model.UndeployOptions{DeleteConfig: true})
	undeploy.Status = status
	f.run = undeploy
	f.repo.run = undeploy
	remover := &recordingRemover{}
	f.o.SetManifestRemover(remover)
	return f, remover
}

func TestUndeployQueuedSkipsBuildAndGoesToDeploying(t *testing.T) {
	f, remover := undeployFixture(t, model.StatusQueued)

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusDeploying, f.run.Status)
	require.Zero(t, remover.calls)
	require.Zero(t, f.prepare.calls)
}

func TestUndeployDeployingRemovesResourcesAndSucceeds(t *testing.T) {
	f, remover := undeployFixture(t, model.StatusDeploying)
	var terminal *model.DeploymentRun
	f.o.SetTerminalHook(func(_ context.Context, run *model.DeploymentRun) { terminal = run })

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Equal(t, 1, remover.calls)
	require.Equal(t, "team-a-dev", remover.target.Namespace)
	require.Equal(t, "orders", remover.spec.AppLabel)
	require.True(t, remover.spec.DeleteConfig)
	require.False(t, remover.spec.DeleteStorage)
	require.NotEmpty(t, remover.spec.Manifests)
	require.Zero(t, f.deployer.calls)
	require.Zero(t, f.deployer.restartCalls)
	require.NotNil(t, terminal)
	require.True(t, terminal.IsUndeploy())
}

func TestUndeployRemoverFailureFailsRun(t *testing.T) {
	f, remover := undeployFixture(t, model.StatusDeploying)
	remover.err = errors.New(`delete Deployment "orders": forbidden`)

	err := f.o.Step(context.Background(), f.run.ID, f.wsID)

	require.Error(t, err)
	require.Equal(t, 1, remover.calls)
	require.NotEqual(t, model.StatusSucceeded, f.run.Status)
}

func TestUndeployWithoutRemoverFailsBusiness(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	baseline := *f.run
	undeploy := model.NewUndeployRun(f.wsID, f.run.ProjectID, f.run.CreatedBy, &baseline, model.UndeployOptions{})
	undeploy.Status = model.StatusDeploying
	f.run = undeploy
	f.repo.run = undeploy

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Zero(t, f.deployer.calls)
}
