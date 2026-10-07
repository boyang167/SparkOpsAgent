package model_test

import (
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/stretchr/testify/require"
)

func TestDiffReleasePlanVersionSnapshots(t *testing.T) {
	now := time.Now().UTC()
	before := model.ReleasePlanVersionSnapshot{
		Name: "v2.3", Description: "old",
		ReleaseWindowStart: now, ReleaseWindowEnd: now.Add(24 * time.Hour),
		ApprovalConfig: model.ApprovalConfig{Enabled: false},
		Jobs: []model.ReleasePlanVersionJobSnapshot{
			{ID: "job-1", Name: "检查项 A", Type: model.ReleasePlanJobTypeText, SortOrder: 0},
		},
	}
	after := before
	after.Name = "v2.4"
	after.Jobs = []model.ReleasePlanVersionJobSnapshot{
		{ID: "job-2", Name: "部署", Type: model.ReleasePlanJobTypeWorkflow, SortOrder: 0},
		{ID: "job-1", Name: "检查项 A", Type: model.ReleasePlanJobTypeText, SortOrder: 1},
	}

	diff := model.DiffReleasePlanVersionSnapshots("plan-1", 2, 1, before, after)
	require.Equal(t, 2, diff.Version)
	require.NotEmpty(t, diff.Groups)

	var hasNameChange, hasOrderChange, hasAdded bool
	for _, group := range diff.Groups {
		for _, change := range group.Changes {
			switch change.ChangeType {
			case "field_changed":
				if change.Path == "name" {
					hasNameChange = true
				}
			case "order_changed":
				hasOrderChange = true
			case "job_added":
				hasAdded = true
			}
		}
	}
	require.True(t, hasNameChange)
	require.True(t, hasOrderChange)
	require.True(t, hasAdded)
}

func TestDiffReleasePlanVersionCreateSnapshot(t *testing.T) {
	now := time.Now().UTC()
	after := model.ReleasePlanVersionSnapshot{
		Name: "new plan",
		ReleaseWindowStart: now, ReleaseWindowEnd: now.Add(24 * time.Hour),
		ApprovalConfig: model.ApprovalConfig{Enabled: true},
	}
	diff := model.DiffReleasePlanVersionSnapshots("plan-1", 1, 0, model.ReleasePlanVersionSnapshot{}, after)
	require.Equal(t, 0, diff.PreviousVersion)
	require.NotEmpty(t, diff.Groups[0].Changes)
}
