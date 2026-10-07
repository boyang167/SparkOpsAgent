package service

import (
	"context"
	"encoding/json"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func MaterializeWorkflowSnapshots(ctx context.Context, plan *model.ReleasePlan, reader port.WorkflowSnapshotReader) error {
	if plan == nil || reader == nil {
		return nil
	}
	for i := range plan.Jobs {
		job := &plan.Jobs[i]
		if job.Type != model.ReleasePlanJobTypeWorkflow || len(job.WorkflowRef) == 0 {
			continue
		}
		var spec model.WorkflowRefSpec
		if err := json.Unmarshal(job.WorkflowRef, &spec); err != nil {
			return err
		}
		if spec.Mode == "snapshot" {
			continue
		}
		if spec.Mode == "" {
			spec.Mode = "reference"
		}
		workflowID, err := kernel.ParseProductWorkflowID(spec.WorkflowID)
		if err != nil {
			return err
		}
		snapshot, err := reader.GetWorkflowSnapshot(ctx, workflowID)
		if err != nil {
			return err
		}
		spec.Mode = "snapshot"
		spec.WorkflowSnapshot = snapshot
		raw, err := json.Marshal(spec)
		if err != nil {
			return err
		}
		job.WorkflowRef = raw
	}
	return nil
}
