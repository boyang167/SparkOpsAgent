package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestReleasePlanHookFlow(t *testing.T) {
	now := time.Now().UTC()
	plan := model.NewReleasePlan(
		kernel.NewProductID(), "hook plan", "",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(24*time.Hour),
		model.ApprovalConfig{Enabled: false},
	)
	plan.HookSettings = model.ReleasePlanHookSettings{
		Enabled: true,
		Events: map[model.ReleasePlanHookEvent]model.ReleasePlanHookEventConfig{
			model.ReleasePlanHookEventBeforeExecute: {Enabled: true, URL: "https://example.com/hook"},
		},
	}
	require.NoError(t, plan.FinishPlanning())
	require.Equal(t, model.ReleasePlanStatusWaitExecuteExternalCheck, plan.Status)
}

func TestParseWorkflowRefSnapshotMode(t *testing.T) {
	now := time.Now().UTC()
	plan := model.NewReleasePlan(
		kernel.NewProductID(), "snap", "",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(time.Hour),
		model.ApprovalConfig{},
	)
	raw, _ := json.Marshal(model.WorkflowRefSpec{
		Mode: "snapshot", WorkflowID: kernel.NewProductWorkflowID().String(),
		WorkflowSnapshot: json.RawMessage(`{"name":"wf"}`),
	})
	plan.Jobs = []model.ReleasePlanJob{{WorkflowRef: raw, Type: model.ReleasePlanJobTypeWorkflow}}
	spec, err := plan.ParseWorkflowRef(&plan.Jobs[0])
	require.NoError(t, err)
	require.Equal(t, "snapshot", spec.Mode)
}
