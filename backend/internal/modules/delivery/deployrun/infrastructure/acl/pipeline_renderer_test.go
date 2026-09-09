package acl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	workspacequery "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type recordingPipelineGenerator struct {
	workspaceID kernel.WorkspaceID
	projectID   kernel.ProjectID
	input       workspacequery.DeploymentPipelineRenderInput
	content     string
}

func (g *recordingPipelineGenerator) RenderDeploymentPipeline(
	_ context.Context,
	workspaceID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input workspacequery.DeploymentPipelineRenderInput,
) (string, error) {
	g.workspaceID = workspaceID
	g.projectID = projectID
	g.input = input
	return g.content, nil
}

func TestPipelineRendererUsesWorkspaceTemplateBoundary(t *testing.T) {
	workspaceID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	generator := &recordingPipelineGenerator{content: "pipeline { echo 'rendered' }"}
	renderer := NewPipelineRenderer(generator)
	request := deployport.PipelineRenderRequest{
		WorkspaceID:          workspaceID,
		ProjectID:            projectID,
		GitRefType:           "tag",
		GitRefValue:          "release/2026-08",
		ImageTag:             "deploy-123",
		ImageRepository:      "registry.example/team-a/orders",
		GitCredentialID:      "sparkops-ws-git",
		RegistryCredentialID: "sparkops-ws-harbor",
	}

	content, err := renderer.Render(context.Background(), request)

	require.NoError(t, err)
	require.Equal(t, generator.content, content)
	require.Equal(t, workspaceID, generator.workspaceID)
	require.Equal(t, projectID, generator.projectID)
	require.Equal(t, workspacequery.DeploymentPipelineRenderInput{
		GitRefType:           request.GitRefType,
		GitRefValue:          request.GitRefValue,
		ImageTag:             request.ImageTag,
		ImageRepository:      request.ImageRepository,
		GitCredentialID:      request.GitCredentialID,
		RegistryCredentialID: request.RegistryCredentialID,
	}, generator.input)
}

func TestNewWorkspacePipelineRendererBuildsProductionAdapter(t *testing.T) {
	require.NotNil(t, NewWorkspacePipelineRenderer(nil))
}
