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

func buildRowProjectIDs(job model.WorkflowJob) []string {
	rows, _ := job.Spec["project_builds"].([]any)
	out := make([]string, 0, len(rows))
	for _, raw := range rows {
		row := raw.(map[string]any)
		out = append(out, row["project_id"].(string))
	}
	return out
}
