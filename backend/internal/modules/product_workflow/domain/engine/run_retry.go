package engine

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// RetryRun resumes a failed run from its first unfinished stage.
// Jobs and per-project items that already succeeded are kept; only failed work is triggered again.
func (s *Scheduler) RetryRun(ctx context.Context, runID kernel.ProductWorkflowRunID) error {
	unlock := s.locker.lock(runID)
	defer unlock()

	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusFailed {
		return domain.ErrRunNotRetryable
	}
	wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return err
	}
	stages := WorkflowStagesForRun(wf, run)
	if !stagesMatchRun(stages, run.StageRuns) {
		return domain.ErrRunDefinitionChanged
	}
	_ = s.syncExternalStatusesOnRun(ctx, run)

	from := len(run.StageRuns)
	for si := range run.StageRuns {
		if !s.stageComplete(run, si) {
			from = si
			break
		}
	}
	if from >= len(run.StageRuns) {
		return domain.ErrRunNotRetryable
	}
	ResetRunForRetry(run, from, time.Now().UTC())
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.reportGit(ctx, run)
	return s.executeCurrentStage(ctx, wf, run)
}

func stagesMatchRun(stages []model.WorkflowStage, stageRuns []model.StageRun) bool {
	if len(stages) != len(stageRuns) {
		return false
	}
	for si := range stages {
		if len(stages[si].Jobs) != len(stageRuns[si].Jobs) {
			return false
		}
		for ji := range stages[si].Jobs {
			if stages[si].Jobs[ji].Name != stageRuns[si].Jobs[ji].Name {
				return false
			}
		}
	}
	return true
}

// ResetRunForRetry puts every unfinished job from stage `from` onward back to pending,
// dropping failed per-project items so executors only re-trigger those.
func ResetRunForRetry(run *model.ProductWorkflowRun, from int, now time.Time) {
	for si := from; si < len(run.StageRuns); si++ {
		stage := &run.StageRuns[si]
		for ji := range stage.Jobs {
			resetJobForRetry(&stage.Jobs[ji])
		}
		stage.Status = model.JobRunStatusPending
	}
	run.CurrentStage = from
	run.Status = model.RunStatusRunning
	run.ErrorMessage = ""
	run.CompletedAt = nil
	run.StartedAt = &now
}

func resetJobForRetry(job *model.JobRun) {
	if job.Status == model.JobRunStatusSuccess || job.Status == model.JobRunStatusSkipped {
		return
	}
	job.Status = model.JobRunStatusPending
	job.ErrorMessage = ""
	job.Approval = nil

	builds := job.Builds[:0]
	for _, item := range job.Builds {
		if item.Status != model.JobRunStatusFailed {
			builds = append(builds, item)
		}
	}
	job.Builds = builds

	versions := job.Versions[:0]
	for _, item := range job.Versions {
		if item.Status != model.JobRunStatusFailed {
			versions = append(versions, item)
		}
	}
	job.Versions = versions

	deploys := job.Deploys[:0]
	for _, item := range job.Deploys {
		if item.Status != model.JobRunStatusFailed {
			deploys = append(deploys, item)
		}
	}
	job.Deploys = deploys
}

// reusableBuilds returns earlier builds of this job that succeeded or are still in flight, keyed by project.
func reusableBuilds(items []model.ProjectBuildRun) map[string]model.ProjectBuildRun {
	out := map[string]model.ProjectBuildRun{}
	for _, item := range items {
		if item.BuildID == "" {
			continue
		}
		if item.Status == model.JobRunStatusSuccess || item.Status == model.JobRunStatusRunning {
			out[item.ProjectID] = item
		}
	}
	return out
}

func reusableDeploys(items []model.ProjectDeployRun) map[string]model.ProjectDeployRun {
	out := map[string]model.ProjectDeployRun{}
	for _, item := range items {
		if item.DeploymentRunID == "" {
			continue
		}
		if item.Status == model.JobRunStatusSuccess || item.Status == model.JobRunStatusRunning {
			out[item.ProjectID] = item
		}
	}
	return out
}

func reusableVersions(items []model.ProjectVersionRun) map[string]model.ProjectVersionRun {
	out := map[string]model.ProjectVersionRun{}
	for _, item := range items {
		if item.VersionID != "" && item.Status == model.JobRunStatusSuccess {
			out[item.ProjectID] = item
		}
	}
	return out
}

func anyBuildRunning(items []model.ProjectBuildRun) bool {
	for _, item := range items {
		if item.Status == model.JobRunStatusRunning || item.Status == model.JobRunStatusPending {
			return true
		}
	}
	return false
}

func anyDeployRunning(items []model.ProjectDeployRun) bool {
	for _, item := range items {
		if item.Status == model.JobRunStatusRunning || item.Status == model.JobRunStatusPending {
			return true
		}
	}
	return false
}
