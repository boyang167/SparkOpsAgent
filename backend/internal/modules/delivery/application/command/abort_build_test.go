package command

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type abortBuildRepo struct {
	build       *model.Build
	updateCalls int
	updateErr   error
}

func (*abortBuildRepo) Save(context.Context, *model.Build) error { return nil }
func (r *abortBuildRepo) Update(context.Context, *model.Build) error {
	r.updateCalls++
	return r.updateErr
}
func (r *abortBuildRepo) FindByID(context.Context, kernel.BuildID) (*model.Build, error) {
	return r.build, nil
}
func (*abortBuildRepo) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.Build, error) {
	return nil, nil
}
func (*abortBuildRepo) HasRunningBuild(context.Context, kernel.ProjectID) (bool, error) {
	return false, nil
}
func (*abortBuildRepo) NextBuildNumber(context.Context, kernel.ProjectID) (int, error) {
	return 1, nil
}

type abortProjectChecker struct{}

func (abortProjectChecker) ProjectInWorkspace(context.Context, kernel.ProjectID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}
func (abortProjectChecker) ProjectIsActive(context.Context, kernel.ProjectID) (bool, error) {
	return true, nil
}
func (abortProjectChecker) ProjectName(context.Context, kernel.ProjectID) (string, error) {
	return "orders", nil
}
func (abortProjectChecker) ProjectNamespace(context.Context, kernel.ProjectID) (string, error) {
	return "team", nil
}
func (abortProjectChecker) ProjectAutomationContext(context.Context, kernel.ProjectID) (port.ProjectAutomationContext, error) {
	return port.ProjectAutomationContext{}, nil
}

func TestAbortBuildCancelsExecutionAndPersistsAbortedStatus(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	build := model.NewBuild(kernel.NewProjectID(), kernel.NewMemberID(), wsID, "main")
	repo := &abortBuildRepo{build: build}
	registry := runner.NewRegistry()
	execCtx, cancel := context.WithCancel(context.Background())
	registry.Register(build.ID.String(), cancel)
	handler := NewAbortBuildHandler(repo, abortProjectChecker{}, registry, stream.NewHub())

	require.NoError(t, handler.Handle(context.Background(), wsID, build.ID))

	require.ErrorIs(t, execCtx.Err(), context.Canceled)
	require.Equal(t, model.BuildStatusAborted, build.Status)
	require.Equal(t, 1, repo.updateCalls)
}
