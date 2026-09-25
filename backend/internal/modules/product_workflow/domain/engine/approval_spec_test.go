package engine

import (
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func TestParseApprovalSpecUsesApproversAndTimeout(t *testing.T) {
	spec := ParseApprovalSpec(map[string]any{
		"provider":           "sparkops",
		"approver_ids":       []any{"m1", "m2"},
		"required_approvals": 2,
		"timeout_minutes":    45,
	})
	require.Equal(t, "sparkops", spec.Provider)
	require.Equal(t, []string{"m1", "m2"}, spec.ApproverIDs)
	require.Equal(t, 2, spec.RequiredApprovals)
	require.Equal(t, 45, spec.TimeoutMinutes)
}

func TestStartApprovalJobStoresRuntimeMetadata(t *testing.T) {
	jobRun := &model.JobRun{}
	StartApprovalJob(jobRun, model.WorkflowJob{
		Type: model.JobTypeSparkopsApproval,
		Spec: map[string]any{
			"approver_ids":       []any{"m1"},
			"required_approvals": 1,
			"timeout_minutes":    30,
		},
	})
	require.Equal(t, model.JobRunStatusRunning, jobRun.Status)
	require.NotNil(t, jobRun.Approval)
	require.Equal(t, model.ApprovalStatusPending, jobRun.Approval.Status)
	require.Equal(t, 1, jobRun.Approval.RequiredApprovals)
	require.Equal(t, 30, jobRun.Approval.TimeoutMinutes)
	require.NotEmpty(t, jobRun.Approval.StartedAt)
}

func TestApprovalTimedOut(t *testing.T) {
	started := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	require.True(t, approvalTimedOut(&model.ApprovalRun{StartedAt: started, TimeoutMinutes: 60}))
	require.False(t, approvalTimedOut(&model.ApprovalRun{StartedAt: time.Now().UTC().Format(time.RFC3339), TimeoutMinutes: 60}))
}
