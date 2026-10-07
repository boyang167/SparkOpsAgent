package model_test

import (
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestReleasePlanTextJobFlow(t *testing.T) {
	now := time.Now().UTC()
	plan := model.NewReleasePlan(
		kernel.NewProductID(), "v2.3 发布", "desc",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(24*time.Hour),
		model.ApprovalConfig{Enabled: true},
	)
	plan.Jobs = []model.ReleasePlanJob{
		{ID: kernel.NewReleasePlanJobID(), Type: model.ReleasePlanJobTypeText, Status: model.ReleasePlanJobStatusTodo},
		{ID: kernel.NewReleasePlanJobID(), Type: model.ReleasePlanJobTypeText, Status: model.ReleasePlanJobStatusTodo},
	}

	require.NoError(t, plan.FinishPlanning())
	require.Equal(t, model.ReleasePlanStatusWaitForApproval, plan.Status)

	require.NoError(t, plan.Approve())
	require.Equal(t, model.ReleasePlanStatusExecuting, plan.Status)

	_, err := plan.MarkTextJobDone(plan.Jobs[0].ID, "")
	require.NoError(t, err)
	require.Equal(t, model.ReleasePlanStatusExecuting, plan.Status)

	needsHook, err := plan.SkipJob(plan.Jobs[1].ID)
	require.NoError(t, err)
	require.False(t, needsHook)
	require.Equal(t, model.ReleasePlanStatusSuccess, plan.Status)
}

func TestWorkflowRefSpecToRunInput(t *testing.T) {
	spec := model.WorkflowRefSpec{
		Mode:       "reference",
		WorkflowID: "wf-1",
		Parameters: map[string]string{
			"branch":                 "main",
			"_environment_overrides": `{"deploy-dev":"env-1","deploy-prod":"env-2"}`,
			"release_note":           "v2.4",
		},
	}

	input := spec.ToRunInput()
	require.Equal(t, "main", input["branch"])

	envOverrides, ok := input["environment_overrides"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "env-1", envOverrides["deploy-dev"])
	require.Equal(t, "env-2", envOverrides["deploy-prod"])

	variables, ok := input["variables"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "v2.4", variables["release_note"])
}

func TestReleasePlanReopenAndCancel(t *testing.T) {
	now := time.Now().UTC()
	plan := model.NewReleasePlan(
		kernel.NewProductID(), "v2.3 发布", "desc",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(24*time.Hour),
		model.ApprovalConfig{Enabled: true},
	)
	require.NoError(t, plan.FinishPlanning())
	require.NoError(t, plan.Reject())
	require.Equal(t, model.ReleasePlanStatusDenied, plan.Status)

	require.NoError(t, plan.ReopenPlanning())
	require.Equal(t, model.ReleasePlanStatusPlanning, plan.Status)

	require.NoError(t, plan.FinishPlanning())
	require.NoError(t, plan.Approve())
	require.NoError(t, plan.Cancel())
	require.Equal(t, model.ReleasePlanStatusCancel, plan.Status)
}

func TestReleasePlanCanDelete(t *testing.T) {
	now := time.Now().UTC()
	planning := model.NewReleasePlan(
		kernel.NewProductID(), "v2.3 发布", "desc",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(24*time.Hour),
		model.ApprovalConfig{Enabled: true},
	)
	require.NoError(t, planning.CanDelete())

	waitApproval := model.NewReleasePlan(
		kernel.NewProductID(), "v2.3 发布", "desc",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(24*time.Hour),
		model.ApprovalConfig{Enabled: true},
	)
	require.NoError(t, waitApproval.FinishPlanning())
	require.Error(t, waitApproval.CanDelete())

	executing := model.NewReleasePlan(
		kernel.NewProductID(), "v2.3 发布", "desc",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(24*time.Hour),
		model.ApprovalConfig{Enabled: false},
	)
	require.NoError(t, executing.FinishPlanning())
	require.Error(t, executing.CanDelete())

	cancelled := model.NewReleasePlan(
		kernel.NewProductID(), "v2.3 发布", "desc",
		kernel.NewMemberID(), kernel.NewMemberID(),
		now, now.Add(24*time.Hour),
		model.ApprovalConfig{Enabled: false},
	)
	require.NoError(t, cancelled.FinishPlanning())
	require.NoError(t, cancelled.Cancel())
	require.NoError(t, cancelled.CanDelete())
}
