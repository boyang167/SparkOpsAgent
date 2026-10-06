package engine_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func TestValidateStages_rejectsCreateVersionWithoutUpstream(t *testing.T) {
	err := engine.ValidateStages([]model.WorkflowStage{
		{
			Name: "构建",
			Jobs: []model.WorkflowJob{{
				Name: "build-1", Type: model.JobTypeSparkopsBuild,
				Spec: map[string]any{"project_builds": []any{map[string]any{"project_id": "p1"}}},
			}},
		},
		{
			Name: "版本",
			Jobs: []model.WorkflowJob{{
				Name: "create-version-2", Type: model.JobTypeSparkopsCreateVersion,
				Spec: map[string]any{"version_tag": "{{version}}", "source": "fromjob", "job_name": ""},
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "create-version-2")
	require.Contains(t, err.Error(), "job_name")
}

func TestValidateStages_allowsCreateVersionWithUpstream(t *testing.T) {
	err := engine.ValidateStages([]model.WorkflowStage{
		{
			Name: "构建",
			Jobs: []model.WorkflowJob{{
				Name: "build-1", Type: model.JobTypeSparkopsBuild,
				Spec: map[string]any{"project_builds": []any{map[string]any{"project_id": "p1"}}},
			}},
		},
		{
			Name: "版本",
			Jobs: []model.WorkflowJob{{
				Name: "create-version-2", Type: model.JobTypeSparkopsCreateVersion,
				Spec: map[string]any{"version_tag": "{{version}}", "source": "fromjob", "job_name": "build-1"},
			}},
		},
	})
	require.NoError(t, err)
}

func TestValidateStages_rejectsDeployWithoutUpstream(t *testing.T) {
	err := engine.ValidateStages([]model.WorkflowStage{
		{
			Name: "部署",
			Jobs: []model.WorkflowJob{{
				Name: "deploy-3", Type: model.JobTypeSparkopsDeploy,
				Spec: map[string]any{"environment_id": "env-1", "source": "fromjob", "job_name": ""},
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "deploy-3")
}
