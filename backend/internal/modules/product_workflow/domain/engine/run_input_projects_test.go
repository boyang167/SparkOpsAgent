package engine

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func TestApplyRunInputFiltersProjects(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name: "release",
		Jobs: []model.WorkflowJob{
			{
				Name: "build",
				Type: model.JobTypeSparkopsBuild,
				Spec: map[string]any{"project_builds": []any{
					map[string]any{"project_id": "p1", "branch": "main"},
					map[string]any{"project_id": "p2", "branch": "main"},
				}},
			},
			{
				Name: "version",
				Type: model.JobTypeSparkopsCreateVersion,
				Spec: map[string]any{"project_ids": []any{"p1", "p2"}},
			},
			{
				Name: "deploy",
				Type: model.JobTypeSparkopsDeploy,
				Spec: map[string]any{"project_builds": []any{
					map[string]any{"project_id": "p1"},
					map[string]any{"project_id": "p2"},
				}},
			},
		},
	}}

	got := ApplyRunInput(stages, map[string]any{"project_ids": []any{"p2"}})

	for _, job := range got[0].Jobs {
		switch job.Name {
		case "build", "deploy":
			require.Equal(t, []string{"p2"}, buildRowProjectIDs(job))
		case "version":
			require.Equal(t, []any{"p2"}, job.Spec["project_ids"])
		}
	}
	// Applying run input must not mutate the persisted workflow definition.
	require.Len(t, stages[0].Jobs[0].Spec["project_builds"], 2)
}

func TestApplyRunInputAddsUnconfiguredProjects(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name: "release",
		Jobs: []model.WorkflowJob{
			{
				Name: "build",
				Type: model.JobTypeSparkopsBuild,
				Spec: map[string]any{"project_builds": []any{
					map[string]any{"project_id": "p1", "branch": "develop"},
				}},
			},
			{
				Name: "version",
				Type: model.JobTypeSparkopsCreateVersion,
				Spec: map[string]any{"project_ids": []any{"p1"}},
			},
		},
	}}

	got := ApplyRunInput(stages, map[string]any{"project_ids": []any{"p1", "p3"}})

	require.Equal(t, []string{"p1", "p3"}, buildRowProjectIDs(got[0].Jobs[0]))
	added := got[0].Jobs[0].Spec["project_builds"].([]any)[1].(map[string]any)
	require.Equal(t, "develop", added["branch"])
	require.Equal(t, []any{"p1", "p3"}, got[0].Jobs[1].Spec["project_ids"])

	withBranch := ApplyRunInput(stages, map[string]any{"project_ids": []any{"p3"}, "branch": "release/1.0"})
	rows := withBranch[0].Jobs[0].Spec["project_builds"].([]any)
	require.Len(t, rows, 1)
	require.Equal(t, "release/1.0", rows[0].(map[string]any)["branch"])
}

func TestApplyRunInputPerProjectBranchesCoverAddedProjects(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name: "release",
		Jobs: []model.WorkflowJob{{
			Name: "build",
			Type: model.JobTypeSparkopsBuild,
			Spec: map[string]any{"project_builds": []any{
				map[string]any{"project_id": "p1", "branch": "main"},
			}},
		}},
	}}

	got := ApplyRunInput(stages, map[string]any{
		"project_ids":      []any{"p1", "p2"},
		"branch":           "develop",
		"branch_overrides":   map[string]any{"p2": "v1.2.0"},
		"ref_type_overrides": map[string]any{"p2": "tag"},
	})

	rows := got[0].Jobs[0].Spec["project_builds"].([]any)
	require.Equal(t, "develop", rows[0].(map[string]any)["branch"])
	require.Equal(t, "v1.2.0", rows[1].(map[string]any)["branch"])
	require.Equal(t, "tag", rows[1].(map[string]any)["git_ref_type"])
}

func buildRowProjectIDs(job model.WorkflowJob) []string {
	rows, _ := job.Spec["project_builds"].([]any)
	out := make([]string, 0, len(rows))
	for _, raw := range rows {
		row := raw.(map[string]any)
		out = append(out, row["project_id"].(string))
	}
	return out
}
