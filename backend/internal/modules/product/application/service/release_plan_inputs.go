package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// ReleasePlanJobInputs resolves the runtime inputs a workflow job still needs after its own overrides apply.
func ReleasePlanJobInputs(
	ctx context.Context,
	plan *model.ReleasePlan,
	job *model.ReleasePlanJob,
	reader appctx.WorkflowRuntimeInputReader,
) (model.WorkflowRefSpec, []appctx.WorkflowRuntimeInput, error) {
	spec, err := plan.ParseWorkflowRef(job)
	if err != nil {
		return spec, nil, err
	}
	if reader == nil {
		return spec, nil, nil
	}
	workflowID, err := kernel.ParseProductWorkflowID(spec.WorkflowID)
	if err != nil {
		return spec, nil, domain.ErrInvalidWorkflowRef
	}
	inputs, err := reader.RuntimeInputs(ctx, plan.ProductID, workflowID, spec.ToRunInput(nil, nil))
	return spec, inputs, err
}

func RuntimeInputKeys(items []appctx.WorkflowRuntimeInput) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.Key)
	}
	return keys
}

// CheckReleasePlanInputs fails when any pending workflow job would start without a required input.
func CheckReleasePlanInputs(ctx context.Context, plan *model.ReleasePlan, reader appctx.WorkflowRuntimeInputReader) error {
	if reader == nil {
		return nil
	}
	problems := []string{}
	for i := range plan.Jobs {
		job := &plan.Jobs[i]
		if job.Type != model.ReleasePlanJobTypeWorkflow || job.Status == model.ReleasePlanJobStatusSkipped {
			continue
		}
		spec, inputs, err := ReleasePlanJobInputs(ctx, plan, job, reader)
		if errors.Is(err, appctx.ErrWorkflowNotFound) {
			return domain.NewReleasePlanWorkflowMissingError(job.Name)
		}
		if err != nil {
			return err
		}
		if missing := spec.MissingInputs(plan.Parameters, RuntimeInputKeys(inputs)); len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("「%s」缺少 %s", job.Name, strings.Join(missing, "、")))
		}
	}
	if len(problems) > 0 {
		return domain.NewReleasePlanMissingInputsError(problems)
	}
	return nil
}
