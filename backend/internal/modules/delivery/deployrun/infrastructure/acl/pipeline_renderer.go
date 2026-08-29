package acl

import (
	"context"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	workspacequery "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	workspaceacl "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/acl"
	workspacepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type deploymentPipelineGenerator interface {
	RenderDeploymentPipeline(
		ctx context.Context,
		workspaceID kernel.WorkspaceID,
		projectID kernel.ProjectID,
		input workspacequery.DeploymentPipelineRenderInput,
	) (string, error)
}

type PipelineRenderer struct {
	generator deploymentPipelineGenerator
}

func NewPipelineRenderer(generator deploymentPipelineGenerator) *PipelineRenderer {
	return &PipelineRenderer{generator: generator}
}

func NewWorkspacePipelineRenderer(db *gorm.DB) *PipelineRenderer {
	return NewPipelineRenderer(workspacequery.NewGenerateProjectTemplatesHandler(
		workspacepersistence.NewProjectRepository(db),
		workspacepersistence.NewTemplateRepository(db),
		workspacepersistence.NewSecretRepository(db),
		workspaceacl.NewTenantACL(db),
		db,
	))
}

func (r *PipelineRenderer) Render(ctx context.Context, request deployport.PipelineRenderRequest) (string, error) {
	return r.generator.RenderDeploymentPipeline(ctx, request.WorkspaceID, request.ProjectID, workspacequery.DeploymentPipelineRenderInput{
		GitRefValue:          request.GitRefValue,
		ImageTag:             request.ImageTag,
		ImageRepository:      request.ImageRepository,
		GitCredentialID:      request.GitCredentialID,
		RegistryCredentialID: request.RegistryCredentialID,
	})
}

var _ deployport.PipelineRenderer = (*PipelineRenderer)(nil)
