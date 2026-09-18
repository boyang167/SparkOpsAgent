package engine

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/stretchr/testify/require"
)

func TestDeployImageUsesBuildOutput(t *testing.T) {
	gitRefType, imageTag, imageRepository := deployImage(model.ProjectBuildRun{
		Branch: "feature/login", GitRefType: "tag", ImageTag: "v1.2.3",
		ImageRepository: "harbor.example/team/orders", Status: model.JobRunStatusSuccess,
	}, "feature/login")

	require.Equal(t, "tag", gitRefType)
	require.Equal(t, "v1.2.3", imageTag)
	require.Equal(t, "harbor.example/team/orders", imageRepository)
}

func TestDeployImageFallsBackToBranchTag(t *testing.T) {
	gitRefType, imageTag, imageRepository := deployImage(model.ProjectBuildRun{
		Branch: "feature/login", Status: model.JobRunStatusSuccess,
	}, "feature/login")

	require.Equal(t, "branch", gitRefType)
	require.Equal(t, "feature-login", imageTag)
	require.Empty(t, imageRepository)
}

func TestResolveFromJobBuildsKeepsSuccessfulOutputs(t *testing.T) {
	run := &model.ProductWorkflowRun{StageRuns: []model.StageRun{{
		Jobs: []model.JobRun{
			{Name: "build-1", Builds: []model.ProjectBuildRun{
				{ProjectID: "p1", Status: model.JobRunStatusSuccess, ImageTag: "main"},
				{ProjectID: "p2", Status: model.JobRunStatusFailed},
			}},
			{Name: "other", Builds: []model.ProjectBuildRun{
				{ProjectID: "p3", Status: model.JobRunStatusSuccess},
			}},
		},
	}}}

	got := resolveFromJobBuilds(run, "build-1")
	require.Len(t, got, 1)
	require.Equal(t, "p1", got[0].ProjectID)
	require.Equal(t, "main", got[0].ImageTag)
}

func TestConfigResourcesForFiltersCurrentProject(t *testing.T) {
	spec := map[string]any{
		"deploy_contents": []any{"image", "config"},
		"config_resources": []any{
			map[string]any{"project_id": "p1", "kind": "ConfigMap", "name": "app-config", "yaml": "kind: ConfigMap"},
			map[string]any{"project_id": "p2", "kind": "Secret", "name": "other"},
		},
	}
	got := configResourcesFor(spec, "p1")
	require.Equal(t, []appctx.WorkflowConfigResource{{Kind: "ConfigMap", Name: "app-config", YAML: "kind: ConfigMap"}}, got)
}

func TestConfigResourcesForIgnoredWhenContentUnchecked(t *testing.T) {
	spec := map[string]any{
		"deploy_contents": []any{"image"},
		"config_resources": []any{
			map[string]any{"project_id": "p1", "kind": "ConfigMap", "name": "app-config"},
		},
	}
	require.Empty(t, configResourcesFor(spec, "p1"))
}
