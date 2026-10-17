package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type gitOperationReporter struct {
	reports int
}

func (r *gitOperationReporter) CurrentStatus(context.Context, workflowoutbound.GitLabStatusPayload) (string, error) {
	return "", nil
}
func (r *gitOperationReporter) Report(context.Context, workflowoutbound.GitLabStatusPayload) error {
	r.reports++
	return nil
}

func TestReportGitSchedulesStableOperationWithoutCallingAdapter(t *testing.T) {
	writer := &notificationOperationWriter{}
	reporter := &gitOperationReporter{}
	scheduler := &Scheduler{operations: writer, git: reporter}
	runID := kernel.NewProductWorkflowRunID()
	trigger, err := json.Marshal(map[string]any{
		"source": "git", "project_id": kernel.NewProjectID().String(),
		"commit": "abc123", "event": "push",
	})
	require.NoError(t, err)
	run := &model.ProductWorkflowRun{ID: runID, Status: model.RunStatusSuccess, Trigger: trigger}
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: "event-1", Subscriber: "product-workflow.build-completed",
	})

	err = scheduler.reportGit(ctx, run)

	require.NoError(t, err)
	require.Zero(t, reporter.reports)
	require.Len(t, writer.requests, 1)
	require.Equal(t, workflowoutbound.OperationTypeGitLabStatusReport, writer.requests[0].OperationType)
	require.Equal(
		t,
		"event-1:product-workflow.build-completed:"+workflowoutbound.OperationTypeGitLabStatusReport+":"+
			runID.String()+"/abc123/sparkops-workflow/success",
		writer.requests[0].IdempotencyKey,
	)
}

func TestGitLabStatusHandlerSkipsDesiredCurrentState(t *testing.T) {
	reporter := &gitOperationReporter{}
	payload, err := json.Marshal(workflowoutbound.GitLabStatusPayload{
		RunID:         kernel.NewProductWorkflowRunID().String(),
		ProjectID:     kernel.NewProjectID().String(),
		Commit:        "abc123",
		Context:       "sparkops/workflow",
		DesiredStatus: "success",
	})
	require.NoError(t, err)
	reporterWithState := &gitReporterWithState{gitOperationReporter: reporter, current: "success"}
	handler := workflowoutbound.NewGitLabStatusHandler(reporterWithState)

	require.NoError(t, handler(context.Background(), messaging.Operation{Payload: payload}))
	require.Zero(t, reporter.reports)
}

type gitReporterWithState struct {
	*gitOperationReporter
	current string
}

func (r *gitReporterWithState) CurrentStatus(context.Context, workflowoutbound.GitLabStatusPayload) (string, error) {
	return r.current, nil
}
