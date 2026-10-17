package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type pipelineProjectRepo struct {
	project *model.Project
}

func (r *pipelineProjectRepo) Save(context.Context, *model.Project) error { return nil }
func (r *pipelineProjectRepo) Update(_ context.Context, project *model.Project) error {
	r.project = project
	return nil
}
func (r *pipelineProjectRepo) FindByID(_ context.Context, id kernel.ProjectID) (*model.Project, error) {
	if r.project == nil || r.project.ID != id {
		return nil, domain.ErrProjectNotFound
	}
	return r.project, nil
}
func (r *pipelineProjectRepo) FindByWorkspaceAndName(context.Context, kernel.WorkspaceID, model.ProjectName) (*model.Project, error) {
	return nil, domain.ErrProjectNotFound
}
func (r *pipelineProjectRepo) ListByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.Project, error) {
	return nil, nil
}
func (r *pipelineProjectRepo) ListByProduct(context.Context, kernel.ProductID) ([]*model.Project, error) {
	return nil, nil
}
func (r *pipelineProjectRepo) CountByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r *pipelineProjectRepo) CountByProduct(context.Context, kernel.ProductID) (int, error) {
	return 0, nil
}
func (r *pipelineProjectRepo) ListByStatus(context.Context, model.ProjectStatus) ([]*model.Project, error) {
	return nil, nil
}

type pipelineDisabler struct {
	disabled   bool
	disableErr error
	calls      int
}

func (d *pipelineDisabler) PipelineDisabled(context.Context, kernel.WorkspaceID, kernel.ProjectID) (bool, error) {
	return d.disabled, nil
}
func (d *pipelineDisabler) DisablePipeline(context.Context, kernel.WorkspaceID, kernel.ProjectID) error {
	d.calls++
	if d.disableErr == nil {
		d.disabled = true
	}
	return d.disableErr
}

func TestPipelineDisableRetriesWithoutPrematureOrDuplicateArchive(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	repo := &pipelineProjectRepo{project: project}
	disabler := &pipelineDisabler{disableErr: errors.New("jenkins unavailable")}
	handler := NewPipelineDisableHandler(func() PipelineDisabler { return disabler }, repo, postgres.NewDirectTransactor())
	payload, err := json.Marshal(PipelineDisablePayload{
		WorkspaceID: project.WorkspaceID.String(), ProjectID: project.ID.String(),
	})
	require.NoError(t, err)
	operation := messaging.Operation{Payload: payload, IdempotencyKey: "event:owner:type:target"}

	require.Error(t, handler(context.Background(), operation))
	require.Equal(t, model.ProjectStatusArchiving, project.Status)

	disabler.disableErr = nil
	require.NoError(t, handler(context.Background(), operation))
	require.Equal(t, model.ProjectStatusArchived, project.Status)
	require.Equal(t, 2, disabler.calls)

	require.NoError(t, handler(context.Background(), operation))
	require.Equal(t, 2, disabler.calls)
}
