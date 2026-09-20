package engine_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func TestStageNeedsManualConfirm(t *testing.T) {
	require.True(t, engine.StageNeedsManualConfirm(model.WorkflowStage{ExecMode: model.StageExecManual}, model.StageRun{}))
	require.False(t, engine.StageNeedsManualConfirm(model.WorkflowStage{ExecMode: model.StageExecManual}, model.StageRun{ManualConfirmed: true}))
	require.False(t, engine.StageNeedsManualConfirm(model.WorkflowStage{ExecMode: model.StageExecAuto}, model.StageRun{}))
	require.False(t, engine.StageNeedsManualConfirm(model.WorkflowStage{}, model.StageRun{}))
}

func TestValidateStages_allowsEmptyJobs(t *testing.T) {
	err := engine.ValidateStages([]model.WorkflowStage{
		{Name: "阶段1", Parallel: true, Jobs: nil},
	})
	require.NoError(t, err)
}

func TestValidateStages_rejectsUnknownType(t *testing.T) {
	err := engine.ValidateStages([]model.WorkflowStage{
		{Name: "s1", Jobs: []model.WorkflowJob{{Name: "j1", Type: "unknown"}}},
	})
	require.Error(t, err)
}

func TestApplyRunInput_overridesBranchAndEnv(t *testing.T) {
	stages := []model.WorkflowStage{
		{
			Name: "build",
			Jobs: []model.WorkflowJob{
				{
					Name: "build-all",
					Type: model.JobTypeSparkopsBuild,
					Spec: map[string]any{
						"project_builds": []any{
							map[string]any{"project_id": "p1", "branch": "main"},
						},
					},
				},
			},
		},
		{
			Name: "deploy",
			Jobs: []model.WorkflowJob{
				{Name: "deploy-uat", Type: model.JobTypeSparkopsDeploy, Spec: map[string]any{"environment_id": "old"}},
			},
		},
	}
	out := engine.ApplyRunInput(stages, map[string]any{
		"branch": "release",
		"environment_overrides": map[string]any{
			"deploy-uat": "env-new",
		},
	})
	require.Equal(t, "release", out[0].Jobs[0].Spec["branch"])
	builds := out[0].Jobs[0].Spec["project_builds"].([]any)
	require.Equal(t, "release", builds[0].(map[string]any)["branch"])
	require.Equal(t, "env-new", out[1].Jobs[0].Spec["environment_id"])
	require.Equal(t, "main", stages[0].Jobs[0].Spec["project_builds"].([]any)[0].(map[string]any)["branch"])
}

func TestApplyRunInput_overridesBranchPerProject(t *testing.T) {
	stages := []model.WorkflowStage{
		{
			Name: "build",
			Jobs: []model.WorkflowJob{
				{
					Name: "build-all",
					Type: model.JobTypeSparkopsBuild,
					Spec: map[string]any{
						"branch": "main",
						"project_builds": []any{
							map[string]any{"project_id": "p1", "branch": "main"},
							map[string]any{"project_id": "p2", "branch": "main"},
						},
					},
				},
			},
		},
	}
	out := engine.ApplyRunInput(stages, map[string]any{
		"branch": "release",
		"branch_overrides": map[string]any{
			"build-all:p1": "feature/login",
			"p2":           "hotfix",
		},
	})
	builds := out[0].Jobs[0].Spec["project_builds"].([]any)
	require.Equal(t, "feature/login", builds[0].(map[string]any)["branch"])
	require.Equal(t, "hotfix", builds[1].(map[string]any)["branch"])
}

func TestApplyRunInput_overridesRefTypePerProject(t *testing.T) {
	stages := []model.WorkflowStage{
		{
			Name: "build",
			Jobs: []model.WorkflowJob{
				{
					Name: "build-all",
					Type: model.JobTypeSparkopsBuild,
					Spec: map[string]any{
						"branch": "main",
						"project_builds": []any{
							map[string]any{"project_id": "p1", "branch": "main", "git_ref_type": "branch"},
							map[string]any{"project_id": "p2", "branch": "main", "git_ref_type": "branch"},
						},
					},
				},
			},
		},
	}
	out := engine.ApplyRunInput(stages, map[string]any{
		"branch_overrides": map[string]any{
			"build-all:p1": "10",
		},
		"ref_type_overrides": map[string]any{
			"build-all:p1": "pr",
		},
	})
	builds := out[0].Jobs[0].Spec["project_builds"].([]any)
	require.Equal(t, "10", builds[0].(map[string]any)["branch"])
	require.Equal(t, "pr", builds[0].(map[string]any)["git_ref_type"])
	require.Equal(t, "main", builds[1].(map[string]any)["branch"])
	require.Equal(t, "branch", builds[1].(map[string]any)["git_ref_type"])
}
