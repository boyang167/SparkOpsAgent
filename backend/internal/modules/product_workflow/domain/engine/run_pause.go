package engine

import (
	"context"
	"encoding/json"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const triggerPauseKindUser = "user"

func triggerMap(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func writeTrigger(run *model.ProductWorkflowRun, trigger map[string]any) {
	raw, err := json.Marshal(trigger)
	if err != nil {
		return
	}
	run.Trigger = raw
}

func setUserPause(run *model.ProductWorkflowRun) {
	trigger := triggerMap(run.Trigger)
	trigger["pause_kind"] = triggerPauseKindUser
	writeTrigger(run, trigger)
}

func clearUserPause(run *model.ProductWorkflowRun) {
	trigger := triggerMap(run.Trigger)
	delete(trigger, "pause_kind")
	writeTrigger(run, trigger)
}

func IsUserPaused(run *model.ProductWorkflowRun) bool {
	if run == nil || run.Status != model.RunStatusPaused {
		return false
	}
	kind, _ := triggerMap(run.Trigger)["pause_kind"].(string)
	return kind == triggerPauseKindUser
}

func PauseKind(run *model.ProductWorkflowRun) string {
	if run == nil {
		return ""
	}
	kind, _ := triggerMap(run.Trigger)["pause_kind"].(string)
	return kind
}

func runAllowsProgress(run *model.ProductWorkflowRun) bool {
	return run != nil && run.Status == model.RunStatusRunning
}

func (s *Scheduler) PauseRun(ctx context.Context, runID kernel.ProductWorkflowRunID) error {
	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusRunning {
		return domain.ErrRunNotPausable
	}
	run.Status = model.RunStatusPaused
	setUserPause(run)
	return s.repo.UpdateRun(ctx, run)
}

func (s *Scheduler) ResumeRun(ctx context.Context, runID kernel.ProductWorkflowRunID) error {
	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusPaused {
		return domain.ErrRunNotPaused
	}
	if IsUserPaused(run) {
		run.Status = model.RunStatusRunning
		clearUserPause(run)
		if err := s.repo.UpdateRun(ctx, run); err != nil {
			return err
		}
		wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
		if err != nil {
			return err
		}
		if s.stageComplete(run, run.CurrentStage) {
			return s.advanceStage(ctx, wf, run)
		}
		return s.executeCurrentStage(ctx, wf, run)
	}
	return s.ContinueManualStage(ctx, runID)
}
