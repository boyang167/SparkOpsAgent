package model

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestCheckStageEntryWipLimit(t *testing.T) {
	stageID := "stage-a"
	sprint := &Sprint{
		Stages: []SprintStage{{ID: stageID, WipLimit: 1}},
	}
	items := []*WorkItem{
		{ID: kernel.NewWorkItemID(), StageID: stageID},
	}
	moving := &WorkItem{ID: kernel.NewWorkItemID(), StageID: "other"}
	err := CheckStageEntry(sprint, moving, stageID, items, nil)
	require.ErrorIs(t, err, domain.ErrStageGateDenied)
}

func TestCheckStageEntryRequireBranch(t *testing.T) {
	stageID := "stage-a"
	sprint := &Sprint{
		Stages: []SprintStage{{ID: stageID, EntryPolicy: &StageGatePolicy{RequireBranch: true}}},
	}
	item := &WorkItem{StageID: "other", Delivery: map[string]any{}}
	err := CheckStageEntry(sprint, item, stageID, nil, nil)
	require.ErrorIs(t, err, domain.ErrStageGateDenied)

	item.Delivery["branch"] = "main"
	require.NoError(t, CheckStageEntry(sprint, item, stageID, nil, nil))
}

func TestCheckStageExitRequiredWorkflow(t *testing.T) {
	wfID := kernel.NewProductWorkflowID()
	sprint := &Sprint{
		Stages: []SprintStage{
			{ID: "todo", SortOrder: 0},
			{ID: "dev", SortOrder: 1, Workflows: []StageWorkflowBinding{{WorkflowID: wfID.String(), RequiredForExit: true}}},
			{ID: "done", SortOrder: 2},
		},
	}
	item := &WorkItem{StageID: "dev"}

	require.ErrorIs(t, CheckStageExit(sprint, item, "done", nil), domain.ErrStageExitDenied)
	require.NoError(t, CheckStageExit(sprint, item, "todo", nil), "moving back is allowed")

	otherStage := []*WorkItemExecution{{StageID: "todo", WorkflowID: wfID, Status: WorkItemExecutionSuccess}}
	require.ErrorIs(t, CheckStageExit(sprint, item, "done", otherStage), domain.ErrStageExitDenied)

	ok := []*WorkItemExecution{{StageID: "dev", WorkflowID: wfID, Status: WorkItemExecutionSuccess}}
	require.NoError(t, CheckStageExit(sprint, item, "done", ok))
}
