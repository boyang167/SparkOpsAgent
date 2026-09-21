package query

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestRunSummaryViewIncludesRemarkAndBranches(t *testing.T) {
	started := time.Now().UTC().Add(-47 * time.Second)
	completed := time.Now().UTC()
	input, _ := json.Marshal(map[string]any{
		"remark": "hotfix",
		"branch_overrides": map[string]any{
			"build-1:p1": "main",
		},
		"ref_type_overrides": map[string]any{
			"build-1:p1": "branch",
		},
	})
	trigger, _ := json.Marshal(map[string]any{"source": "manual", "started_by": "member-1"})
	run := &model.ProductWorkflowRun{
		ID: kernel.NewProductWorkflowRunID(), Status: model.RunStatusSuccess,
		Input: input, Trigger: trigger, CurrentStage: 1,
		StartedAt: &started, CompletedAt: &completed, CreatedAt: started,
		StageRuns: []model.StageRun{{
			Jobs: []model.JobRun{{
				Type: model.JobTypeSparkopsBuild,
				Builds: []model.ProjectBuildRun{{ProjectID: "p1", Branch: "main", Status: model.JobRunStatusSuccess}},
			}},
		}},
	}
	view := runSummaryView(run, 99)
	require.Equal(t, 99, view["run_number"])
	require.Equal(t, "hotfix", view["remark"])
	require.Equal(t, "manual", view["trigger_source"])
	require.Equal(t, "member-1", view["executor_id"])
	require.NotNil(t, view["branches"])
	require.NotNil(t, view["build_services"])
	require.Greater(t, view["duration_ms"].(int64), int64(0))
}
