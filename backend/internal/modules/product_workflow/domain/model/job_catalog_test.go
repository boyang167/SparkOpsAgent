package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func TestJobCatalog_registersExecutableSparkOpsJobs(t *testing.T) {
	got := map[string]model.JobCatalogItem{}
	categories := map[string]struct{}{}
	for _, item := range model.JobCatalog() {
		require.True(t, item.Executable, item.Type)
		got[item.Type] = item
		categories[item.Category] = struct{}{}
	}
	require.Len(t, categories, 4)
	require.Equal(t, []string{"build", "deploy", "flow", "tools"}, keysOf(categories))
	require.Equal(t, "SparkOps 构建", got[model.JobTypeSparkopsBuild].Title)
	require.Equal(t, "SparkOps 部署", got[model.JobTypeSparkopsDeploy].Title)
	require.Equal(t, "人工审批", got[model.JobTypeSparkopsApproval].Title)
	require.Equal(t, "审批通过后才可执行后续操作", got[model.JobTypeSparkopsApproval].Description)
	require.Equal(t, "自由脚本", got[model.JobTypeFreestyle].Title)
	require.Equal(t, "通知", got[model.JobTypeNotify].Title)
	require.True(t, model.IsKnownJobType(model.JobTypeSparkopsBuild))
	require.False(t, model.IsKnownJobType("zadig-build"))
	require.False(t, model.IsKnownJobType("jenkins"))
}

func keysOf(set map[string]struct{}) []string {
	order := []string{"build", "deploy", "flow", "tools"}
	out := make([]string, 0, len(order))
	for _, key := range order {
		if _, ok := set[key]; ok {
			out = append(out, key)
		}
	}
	return out
}
