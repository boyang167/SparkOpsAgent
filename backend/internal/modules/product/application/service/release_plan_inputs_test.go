package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type fakeInputReader struct {
	byWorkflow map[string][]appctx.WorkflowRuntimeInput
	err        error
}

func (f fakeInputReader) RuntimeInputs(_ context.Context, _ kernel.ProductID, id kernel.ProductWorkflowID, _ map[string]any) ([]appctx.WorkflowRuntimeInput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byWorkflow[id.String()], nil
}

func planningPlanWithJob(t *testing.T, name string, workflowID kernel.ProductWorkflowID, params map[string]string) *model.ReleasePlan {
	t.Helper()
	now := time.Now().UTC()
	plan := model.NewReleasePlan(kernel.NewProductID(), "p", "", kernel.NewMemberID(), kernel.NewMemberID(), now, now.Add(time.Hour), model.ApprovalConfig{})
	ref, _ := json.Marshal(model.WorkflowRefSpec{Mode: "reference", WorkflowID: workflowID.String(), Parameters: params})
	plan.Jobs = []model.ReleasePlanJob{{
		ID: kernel.NewReleasePlanJobID(), Name: name, Type: model.ReleasePlanJobTypeWorkflow,
		Status: model.ReleasePlanJobStatusTodo, WorkflowRef: ref,
	}}
	return plan
}

func TestCheckReleasePlanInputsReportsMissingKeys(t *testing.T) {
	wfID := kernel.NewProductWorkflowID()
	reader := fakeInputReader{byWorkflow: map[string][]appctx.WorkflowRuntimeInput{
		wfID.String(): {{Key: "version"}, {Key: "ticket"}},
	}}
	plan := planningPlanWithJob(t, "dev", wfID, map[string]string{"ticket": "OPS-1"})

	err := service.CheckReleasePlanInputs(context.Background(), plan, reader)

	require.Error(t, err)
	de, ok := kernel.IsDomainError(err)
	require.True(t, ok)
	require.Equal(t, "RELEASE_PLAN_MISSING_INPUTS", de.Code())
	require.Contains(t, err.Error(), "「dev」缺少 version")
	require.NotContains(t, err.Error(), "ticket")
}

func TestCheckReleasePlanInputsPassesWithPlanParameters(t *testing.T) {
	wfID := kernel.NewProductWorkflowID()
	reader := fakeInputReader{byWorkflow: map[string][]appctx.WorkflowRuntimeInput{wfID.String(): {{Key: "version"}}}}
	plan := planningPlanWithJob(t, "dev", wfID, nil)
	plan.Parameters = map[string]string{"version": "1.2.0"}

	require.NoError(t, service.CheckReleasePlanInputs(context.Background(), plan, reader))
}

func TestCheckReleasePlanInputsSkipsSkippedJobs(t *testing.T) {
	wfID := kernel.NewProductWorkflowID()
	reader := fakeInputReader{byWorkflow: map[string][]appctx.WorkflowRuntimeInput{wfID.String(): {{Key: "version"}}}}
	plan := planningPlanWithJob(t, "dev", wfID, nil)
	plan.Jobs[0].Status = model.ReleasePlanJobStatusSkipped

	require.NoError(t, service.CheckReleasePlanInputs(context.Background(), plan, reader))
}

func TestCheckReleasePlanInputsNamesMissingWorkflow(t *testing.T) {
	plan := planningPlanWithJob(t, "dev", kernel.NewProductWorkflowID(), nil)

	err := service.CheckReleasePlanInputs(context.Background(), plan, fakeInputReader{err: appctx.ErrWorkflowNotFound})

	require.EqualError(t, err, "发布项「dev」关联的工作流不存在")
}

func TestCheckReleasePlanInputsWithoutReaderIsNoop(t *testing.T) {
	plan := planningPlanWithJob(t, "dev", kernel.NewProductWorkflowID(), nil)
	require.NoError(t, service.CheckReleasePlanInputs(context.Background(), plan, nil))
}
