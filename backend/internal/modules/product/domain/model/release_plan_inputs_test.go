package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func executingWorkflowPlan(t *testing.T) *model.ReleasePlan {
	t.Helper()
	now := time.Now().UTC()
	plan := model.NewReleasePlan(
		kernel.NewProductID(), "v2.4 发布", "", kernel.NewMemberID(), kernel.NewMemberID(),
		now.Add(-time.Hour), now.Add(time.Hour), model.ApprovalConfig{},
	)
	ref, _ := json.Marshal(model.WorkflowRefSpec{Mode: "reference", WorkflowID: kernel.NewProductWorkflowID().String()})
	plan.Jobs = []model.ReleasePlanJob{{
		ID: kernel.NewReleasePlanJobID(), Name: "dev", Type: model.ReleasePlanJobTypeWorkflow,
		Status: model.ReleasePlanJobStatusTodo, WorkflowRef: ref,
	}}
	require.NoError(t, plan.FinishPlanning())
	require.Equal(t, model.ReleasePlanStatusExecuting, plan.Status)
	return plan
}

func TestToRunInputMergesRequiredPlanParameters(t *testing.T) {
	spec := model.WorkflowRefSpec{Parameters: map[string]string{
		"branch": "main", "release_note": "hotfix", "ticket": "OPS-9",
	}}
	plan := map[string]string{"version": "1.2.0", "ticket": "OPS-1", "unused": "x"}

	input := spec.ToRunInput(plan, []string{"version", "ticket"})

	require.Equal(t, "main", input["branch"])
	require.Equal(t, map[string]any{"release_note": "hotfix", "ticket": "OPS-9", "version": "1.2.0"}, input["variables"])
}

func TestToRunInputWithoutJobParametersStillUsesPlan(t *testing.T) {
	input := model.WorkflowRefSpec{}.ToRunInput(map[string]string{"version": "1.2.0"}, []string{"version"})
	require.Equal(t, map[string]any{"version": "1.2.0"}, input["variables"])
}

func TestMissingInputsIgnoresBlankValuesAndReservedJobKeys(t *testing.T) {
	spec := model.WorkflowRefSpec{Parameters: map[string]string{"branch": "main", "env": " "}}
	missing := spec.MissingInputs(map[string]string{"env": ""}, []string{"version", "env", "branch"})
	require.Equal(t, []string{"version", "env", "branch"}, missing)

	spec.Parameters["version"] = "1.0.0"
	require.Equal(t, []string{"env"}, spec.MissingInputs(map[string]string{"branch": "main"}, []string{"version", "env", "branch"}))
}

func TestRestoreMaskedParametersKeepsStoredSecrets(t *testing.T) {
	next := map[string]string{"token": model.MaskedParameterValue, "version": "1.2.0", "ghost": model.MaskedParameterValue}
	prev := map[string]string{"token": "s3cret"}
	require.Equal(t, map[string]string{"token": "s3cret", "version": "1.2.0"}, model.RestoreMaskedParameters(next, prev))
}

func TestMaskParametersHidesOnlySecretKeys(t *testing.T) {
	masked := model.MaskParameters(map[string]string{"token": "s3cret", "version": "1.2.0", "empty": ""},
		map[string]struct{}{"token": {}, "empty": {}})
	require.Equal(t, map[string]string{"token": model.MaskedParameterValue, "version": "1.2.0", "empty": ""}, masked)
}

func TestNormalizeParametersTrimsAndDropsBlanks(t *testing.T) {
	require.Equal(t, map[string]string{"version": "1.2.0"},
		model.NormalizeParameters(map[string]string{" version ": " 1.2.0 ", "": "x", "note": "  "}))
}

func TestFailWorkflowJobStartThenRetry(t *testing.T) {
	plan := executingWorkflowPlan(t)
	jobID := plan.Jobs[0].ID

	require.NoError(t, plan.FailWorkflowJobStart(jobID, "缺少运行时必填变量: version"))
	require.Equal(t, model.ReleasePlanJobStatusFailed, plan.Jobs[0].Status)
	var ref model.ExecutionRef
	require.NoError(t, json.Unmarshal(plan.Jobs[0].ExecutionRef, &ref))
	require.Equal(t, "缺少运行时必填变量: version", ref.ErrorMessage)

	require.NoError(t, plan.RetryWorkflowJob(jobID))
	require.Equal(t, model.ReleasePlanJobStatusTodo, plan.Jobs[0].Status)
}

func TestEnsureJobEditable(t *testing.T) {
	plan := executingWorkflowPlan(t)
	jobID := plan.Jobs[0].ID

	require.Error(t, plan.EnsureJobEditable(jobID, true))
	require.NoError(t, plan.FailWorkflowJobStart(jobID, "boom"))
	require.NoError(t, plan.EnsureJobEditable(jobID, true))
	require.Error(t, plan.EnsureJobEditable(jobID, false))
	require.Error(t, plan.EnsureJobEditable(kernel.NewReleasePlanJobID(), true))
}

func TestVersionDiffReportsParameterPresenceOnly(t *testing.T) {
	before := model.ReleasePlanVersionSnapshot{Name: "p", Parameters: map[string]string{"version": "1.0.0", "token": "a"}}
	after := model.ReleasePlanVersionSnapshot{Name: "p", Parameters: map[string]string{"version": "1.1.0", "ticket": "OPS-1"}}

	resp := model.DiffReleasePlanVersionSnapshots("plan-1", 2, 1, before, after)

	raw, _ := json.Marshal(resp)
	require.NotContains(t, string(raw), "1.1.0")
	require.NotContains(t, string(raw), "OPS-1")
	labels := []string{}
	for _, group := range resp.Groups {
		for _, change := range group.Changes {
			labels = append(labels, change.Label)
		}
	}
	require.Equal(t, []string{"发布参数 · ticket", "发布参数 · token", "发布参数 · version"}, labels)
}
