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

	require.NoError(t, plan.MarkTextJobDone(plan.Jobs[0].ID, ""))
	require.Equal(t, model.ReleasePlanStatusExecuting, plan.Status)

	require.NoError(t, plan.SkipJob(plan.Jobs[1].ID))
	require.Equal(t, model.ReleasePlanStatusSuccess, plan.Status)
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
