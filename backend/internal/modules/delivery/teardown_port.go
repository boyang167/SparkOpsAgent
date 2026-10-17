package delivery

import (
	"context"

	deploycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/command"
	deployservice "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const teardownRunWindow = 100

type teardownRunReader interface {
	ListByProject(ctx context.Context, projectID kernel.ProjectID, limit int) ([]*model.DeploymentRun, error)
	HasActiveByProject(ctx context.Context, projectID kernel.ProjectID) (bool, error)
	FindByID(ctx context.Context, runID kernel.DeploymentRunID) (*model.DeploymentRun, error)
}

type teardownPort struct {
	runs     teardownRunReader
	undeploy *deploycommand.UndeployHandler
	reader   deployport.ProjectDeploymentReader
	prepare  *deployservice.PrepareService
}

func (p *teardownPort) OnlineClusters(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]kernel.ClusterInfoID, error) {
	runs, err := p.runs.ListByProject(ctx, projectID, teardownRunWindow)
	if err != nil {
		return nil, err
	}
	scoped := make([]*model.DeploymentRun, 0, len(runs))
	for _, run := range runs {
		if run != nil && run.WorkspaceID == wsID {
			scoped = append(scoped, run)
		}
	}
	return model.OnlineClusters(scoped), nil
}

func (p *teardownPort) HasActiveRun(ctx context.Context, projectID kernel.ProjectID) (bool, error) {
	return p.runs.HasActiveByProject(ctx, projectID)
}

func (p *teardownPort) StartUndeploy(ctx context.Context, req appctx.UndeployRequest) (kernel.DeploymentRunID, error) {
	run, err := p.undeploy.StartForArchive(ctx, req.WorkspaceID, req.MemberID, req.ProjectID, req.ClusterID, req.ArchiveGeneration)
	if err != nil {
		return kernel.DeploymentRunID{}, err
	}
	return run.ID, nil
}

func (p *teardownPort) ArchiveGeneration(ctx context.Context, runID kernel.DeploymentRunID) (string, error) {
	run, err := p.runs.FindByID(ctx, runID)
	if err != nil {
		return "", err
	}
	return run.ArchiveGeneration, nil
}

func (p *teardownPort) DisablePipeline(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
	project, err := p.reader.Load(ctx, wsID, projectID, kernel.ClusterInfoID{})
	if err != nil {
		return err
	}
	return p.prepare.DisableJob(ctx, project)
}

func (p *teardownPort) PipelineDisabled(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (bool, error) {
	project, err := p.reader.Load(ctx, wsID, projectID, kernel.ClusterInfoID{})
	if err != nil {
		return false, err
	}
	return p.prepare.PipelineDisabled(ctx, project)
}

var _ appctx.ProjectTeardown = (*teardownPort)(nil)
