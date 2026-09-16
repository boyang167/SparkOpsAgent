package engine

import (
	"context"
	"encoding/json"
	"time"

	deployrunmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Scheduler struct {
	repo    repository.ProductWorkflowRepository
	builds  *BuildExecutor
	deploys *DeployExecutor
	bus     eventbus.Bus
}

func NewScheduler(repo repository.ProductWorkflowRepository, builds *BuildExecutor, deploys *DeployExecutor, bus eventbus.Bus) *Scheduler {
	return &Scheduler{repo: repo, builds: builds, deploys: deploys, bus: bus}
}

func (s *Scheduler) StartRun(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun) error {
	now := time.Now().UTC()
	run.Status = model.RunStatusRunning
	run.StartedAt = &now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	return s.executeCurrentStage(ctx, wf, run)
}

func (s *Scheduler) HandleBuildCompleted(ctx context.Context, buildID kernel.BuildID, status string) error {
	ref, err := s.repo.FindRunByBuildID(ctx, buildID)
	if err != nil {
		return nil
	}
	run, err := s.repo.FindRunByID(ctx, ref.RunID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusRunning {
		return nil
	}
	wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return err
	}

	updated := false
	for si := range run.StageRuns {
		for ji := range run.StageRuns[si].Jobs {
			jobRun := &run.StageRuns[si].Jobs[ji]
			if jobRun.Name != ref.JobName {
				continue
			}
			for bi := range jobRun.Builds {
				if jobRun.Builds[bi].BuildID != buildID.String() {
					continue
				}
				switch status {
				case string(buildmodel.BuildStatusSuccess):
					jobRun.Builds[bi].Status = model.JobRunStatusSuccess
				case string(buildmodel.BuildStatusFailed), string(buildmodel.BuildStatusAborted):
					jobRun.Builds[bi].Status = model.JobRunStatusFailed
					jobRun.Builds[bi].ErrorMessage = "build " + status
				default:
					return nil
				}
				updated = true
			}
		}
	}
	if !updated {
		return nil
	}
	if status != string(buildmodel.BuildStatusSuccess) {
		return s.failRun(ctx, run, "build failed")
	}
	if failed, msg := s.refreshJobStatuses(run); failed {
		return s.failRun(ctx, run, msg)
	}
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if s.stageComplete(run, run.CurrentStage) {
		return s.advanceStage(ctx, wf, run)
	}
	return nil
}

func (s *Scheduler) HandleDeployCompleted(ctx context.Context, deploymentID kernel.DeploymentRunID, status string) error {
	ref, err := s.repo.FindRunByDeploymentID(ctx, deploymentID)
	if err != nil {
		return nil
	}
	run, err := s.repo.FindRunByID(ctx, ref.RunID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusRunning {
		return nil
	}
	wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return err
	}

	updated := false
	for si := range run.StageRuns {
		for ji := range run.StageRuns[si].Jobs {
			jobRun := &run.StageRuns[si].Jobs[ji]
			if jobRun.Name != ref.JobName {
				continue
			}
			for di := range jobRun.Deploys {
				if jobRun.Deploys[di].DeploymentRunID != deploymentID.String() {
					continue
				}
				switch status {
				case string(deployrunmodel.StatusSucceeded):
					jobRun.Deploys[di].Status = model.JobRunStatusSuccess
				case string(deployrunmodel.StatusFailed), string(deployrunmodel.StatusAborted):
					jobRun.Deploys[di].Status = model.JobRunStatusFailed
					jobRun.Deploys[di].ErrorMessage = "deployment " + status
				default:
					return nil
				}
				updated = true
			}
		}
	}
	if !updated {
		return nil
	}
	if status != string(deployrunmodel.StatusSucceeded) {
		return s.failRun(ctx, run, "deployment failed")
	}
	if failed, msg := s.refreshJobStatuses(run); failed {
		return s.failRun(ctx, run, msg)
	}
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if s.stageComplete(run, run.CurrentStage) {
		return s.advanceStage(ctx, wf, run)
	}
	return nil
}

func (s *Scheduler) ApproveJob(ctx context.Context, runID kernel.ProductWorkflowRunID, jobName string, actor kernel.MemberID) error {
	run, wf, jobRun, err := s.loadPendingApproval(ctx, runID, jobName)
	if err != nil {
		return err
	}
	jobRun.Approval.Status = model.ApprovalStatusApproved
	jobRun.Approval.ActorID = actor.String()
	jobRun.Status = model.JobRunStatusSuccess
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if s.stageComplete(run, run.CurrentStage) {
		return s.advanceStage(ctx, wf, run)
	}
	return nil
}

func (s *Scheduler) RejectJob(ctx context.Context, runID kernel.ProductWorkflowRunID, jobName string, actor kernel.MemberID, message string) error {
	run, _, jobRun, err := s.loadPendingApproval(ctx, runID, jobName)
	if err != nil {
		return err
	}
	jobRun.Approval.Status = model.ApprovalStatusRejected
	jobRun.Approval.ActorID = actor.String()
	jobRun.Approval.Message = message
	jobRun.Status = model.JobRunStatusFailed
	jobRun.ErrorMessage = message
	return s.failRun(ctx, run, "approval rejected")
}

func (s *Scheduler) loadPendingApproval(ctx context.Context, runID kernel.ProductWorkflowRunID, jobName string) (*model.ProductWorkflowRun, *model.ProductWorkflow, *model.JobRun, error) {
	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return nil, nil, nil, err
	}
	if run.Status != model.RunStatusRunning {
		return nil, nil, nil, domain.ErrApprovalNotPending
	}
	wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return nil, nil, nil, err
	}
	if run.CurrentStage >= len(run.StageRuns) {
		return nil, nil, nil, domain.ErrApprovalNotPending
	}
	for ji := range run.StageRuns[run.CurrentStage].Jobs {
		jobRun := &run.StageRuns[run.CurrentStage].Jobs[ji]
		if jobRun.Name == jobName && IsApprovalWaiting(*jobRun) {
			return run, wf, jobRun, nil
		}
	}
	return nil, nil, nil, domain.ErrApprovalNotPending
}

func (s *Scheduler) executeCurrentStage(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun) error {
	if run.CurrentStage >= len(wf.Stages) {
		return s.completeRun(ctx, run)
	}
	stage := wf.Stages[run.CurrentStage]
	stageRun := &run.StageRuns[run.CurrentStage]
	stageRun.Status = model.JobRunStatusRunning

	hasAsync := false
	for ji, job := range stage.Jobs {
		jobRun := &stageRun.Jobs[ji]
		if jobRun.Status == model.JobRunStatusSuccess || jobRun.Status == model.JobRunStatusSkipped {
			continue
		}
		if IsApprovalWaiting(*jobRun) {
			hasAsync = true
			continue
		}
		switch job.Type {
		case model.JobTypeSparkopsBuild:
			hasAsync = true
			if err := s.builds.RunJob(ctx, run, run.CurrentStage, ji, job); err != nil {
				return s.failRun(ctx, run, jobRun.ErrorMessage)
			}
		case model.JobTypeSparkopsDeploy:
			hasAsync = true
			if err := s.deploys.RunJob(ctx, run, run.CurrentStage, ji, job); err != nil {
				return s.failRun(ctx, run, jobRun.ErrorMessage)
			}
		case model.JobTypeSparkopsApproval:
			hasAsync = true
			StartApprovalJob(jobRun, job)
		default:
			jobRun.Status = model.JobRunStatusSkipped
		}
	}
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if !hasAsync && s.stageComplete(run, run.CurrentStage) {
		return s.advanceStage(ctx, wf, run)
	}
	return nil
}

func (s *Scheduler) advanceStage(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun) error {
	run.StageRuns[run.CurrentStage].Status = model.JobRunStatusSuccess
	run.CurrentStage++
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if run.CurrentStage >= len(wf.Stages) {
		return s.completeRun(ctx, run)
	}
	return s.executeCurrentStage(ctx, wf, run)
}

func (s *Scheduler) refreshJobStatuses(run *model.ProductWorkflowRun) (bool, string) {
	for si := range run.StageRuns {
		for ji := range run.StageRuns[si].Jobs {
			jobRun := &run.StageRuns[si].Jobs[ji]
			switch jobRun.Type {
			case model.JobTypeSparkopsBuild:
				if failed, msg := refreshBuildJob(jobRun); failed {
					run.StageRuns[si].Status = model.JobRunStatusFailed
					return true, msg
				}
			case model.JobTypeSparkopsDeploy:
				if failed, msg := refreshDeployJob(jobRun); failed {
					run.StageRuns[si].Status = model.JobRunStatusFailed
					return true, msg
				}
			case model.JobTypeSparkopsApproval:
				if IsApprovalWaiting(*jobRun) {
					continue
				}
				if jobRun.Approval != nil && jobRun.Approval.Status == model.ApprovalStatusRejected {
					jobRun.Status = model.JobRunStatusFailed
					return true, "approval rejected"
				}
				if jobRun.Approval != nil && jobRun.Approval.Status == model.ApprovalStatusApproved {
					jobRun.Status = model.JobRunStatusSuccess
				}
			}
		}
		if s.stageComplete(run, si) {
			run.StageRuns[si].Status = model.JobRunStatusSuccess
		}
	}
	return false, ""
}

func refreshBuildJob(jobRun *model.JobRun) (bool, string) {
	if len(jobRun.Builds) == 0 {
		return false, ""
	}
	allSuccess := true
	anyFailed := false
	for _, b := range jobRun.Builds {
		switch b.Status {
		case model.JobRunStatusFailed:
			anyFailed = true
			allSuccess = false
		case model.JobRunStatusRunning, model.JobRunStatusPending:
			allSuccess = false
		}
	}
	if anyFailed {
		jobRun.Status = model.JobRunStatusFailed
		return true, "build job failed"
	}
	if allSuccess {
		jobRun.Status = model.JobRunStatusSuccess
	}
	return false, ""
}

func refreshDeployJob(jobRun *model.JobRun) (bool, string) {
	if len(jobRun.Deploys) == 0 {
		return false, ""
	}
	allSuccess := true
	anyFailed := false
	for _, d := range jobRun.Deploys {
		switch d.Status {
		case model.JobRunStatusFailed:
			anyFailed = true
			allSuccess = false
		case model.JobRunStatusRunning, model.JobRunStatusPending:
			allSuccess = false
		}
	}
	if anyFailed {
		jobRun.Status = model.JobRunStatusFailed
		return true, "deploy job failed"
	}
	if allSuccess {
		jobRun.Status = model.JobRunStatusSuccess
	}
	return false, ""
}

func (s *Scheduler) stageComplete(run *model.ProductWorkflowRun, stageIndex int) bool {
	if stageIndex >= len(run.StageRuns) {
		return true
	}
	for _, job := range run.StageRuns[stageIndex].Jobs {
		if IsApprovalWaiting(job) {
			return false
		}
		switch job.Status {
		case model.JobRunStatusSuccess, model.JobRunStatusSkipped:
			continue
		default:
			return false
		}
	}
	return true
}

func (s *Scheduler) completeRun(ctx context.Context, run *model.ProductWorkflowRun) error {
	now := time.Now().UTC()
	run.Status = model.RunStatusSuccess
	run.CompletedAt = &now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	return s.publishRunCompleted(ctx, run)
}

func (s *Scheduler) failRun(ctx context.Context, run *model.ProductWorkflowRun, message string) error {
	now := time.Now().UTC()
	run.Status = model.RunStatusFailed
	if run.ErrorMessage == "" {
		run.ErrorMessage = message
	}
	run.CompletedAt = &now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	return s.publishRunCompleted(ctx, run)
}

func (s *Scheduler) publishRunCompleted(ctx context.Context, run *model.ProductWorkflowRun) error {
	if s.bus == nil || run == nil {
		return nil
	}
	wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return nil
	}
	return s.bus.Publish(ctx, pwfevent.NewWorkflowRunCompleted(
		run.ID, run.WorkflowID, run.ProductID, wf.WorkspaceID, string(run.Status), run.ErrorMessage,
	))
}

func ParseStagesFromDefinition(def json.RawMessage) ([]model.WorkflowStage, error) {
	var wrapper struct {
		Stages []model.WorkflowStage `json:"stages"`
	}
	if err := json.Unmarshal(def, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Stages, nil
}

func InjectProjectBuilds(stages []model.WorkflowStage, projectIDs []kernel.ProjectID, defaultBranch string) {
	for si := range stages {
		for ji := range stages[si].Jobs {
			if stages[si].Jobs[ji].Type != model.JobTypeSparkopsBuild {
				continue
			}
			if stages[si].Jobs[ji].Spec == nil {
				stages[si].Jobs[ji].Spec = map[string]any{}
			}
			branch := defaultBranch
			if b, ok := stages[si].Jobs[ji].Spec["branch"].(string); ok && b != "" {
				branch = b
			}
			builds := make([]map[string]any, 0, len(projectIDs))
			for _, pid := range projectIDs {
				builds = append(builds, map[string]any{"project_id": pid.String(), "branch": branch})
			}
			stages[si].Jobs[ji].Spec["project_builds"] = builds
		}
	}
}
