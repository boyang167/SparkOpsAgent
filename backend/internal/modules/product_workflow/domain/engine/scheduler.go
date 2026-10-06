package engine

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	deployrunmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GitReporter interface {
	Report(ctx context.Context, run *model.ProductWorkflowRun)
}

type ExternalStatusReader interface {
	BuildStatus(ctx context.Context, buildID kernel.BuildID) (status string, terminal bool, err error)
	DeployStatus(ctx context.Context, deploymentID kernel.DeploymentRunID) (status string, terminal bool, err error)
}

type Scheduler struct {
	repo           repository.ProductWorkflowRepository
	builds         *BuildExecutor
	createVersions *CreateVersionExecutor
	deploys *DeployExecutor
	bus     eventbus.Bus
	git            GitReporter
	locker         runLocker
	status         ExternalStatusReader
}

func (s *Scheduler) SetGitReporter(reporter GitReporter) {
	if s != nil {
		s.git = reporter
	}
}

func (s *Scheduler) SetStatusReader(reader ExternalStatusReader) {
	if s != nil {
		s.status = reader
	}
}

func (s *Scheduler) reportGit(ctx context.Context, run *model.ProductWorkflowRun) {
	if s == nil || s.git == nil || run == nil {
		return
	}
	s.git.Report(ctx, run)
}

func NewScheduler(
	repo repository.ProductWorkflowRepository,
	builds *BuildExecutor,
	createVersions *CreateVersionExecutor,
	deploys *DeployExecutor,
	bus eventbus.Bus,
) *Scheduler {
	return &Scheduler{
		repo: repo, builds: builds, createVersions: createVersions,
		deploys: deploys, bus: bus,
	}
}

func (s *Scheduler) StartRun(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun) error {
	now := time.Now().UTC()
	run.Status = model.RunStatusRunning
	run.StartedAt = &now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.emitNotification(ctx, wf, run, model.NotifyEventCreated)
	s.reportGit(ctx, run)
	return s.executeCurrentStage(ctx, wf, run)
}

func (s *Scheduler) HandleBuildCompleted(ctx context.Context, buildID kernel.BuildID, status string) error {
	ref, err := s.repo.FindRunByBuildID(ctx, buildID)
	if err != nil {
		return nil
	}
	unlock := s.locker.lock(ref.RunID)
	defer unlock()
	return s.handleBuildCompletedLocked(ctx, ref.RunID, ref.JobName, buildID, status)
}

func (s *Scheduler) handleBuildCompletedLocked(
	ctx context.Context,
	runID kernel.ProductWorkflowRunID,
	jobName string,
	buildID kernel.BuildID,
	status string,
) error {
	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusRunning && run.Status != model.RunStatusPaused {
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
			if jobRun.Name != jobName {
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
		if job, ok := jobByName(wf, jobName); ok {
			s.emitJobNotification(ctx, wf, run, job.Spec, model.NotifyEventFailed)
		}
		return s.failRun(ctx, run, "build failed")
	}
	if failed, msg := s.refreshJobStatuses(run); failed {
		if job, ok := jobByName(wf, jobName); ok {
			s.emitJobNotification(ctx, wf, run, job.Spec, model.NotifyEventFailed)
		}
		return s.failRun(ctx, run, msg)
	}
	if job, ok := jobByName(wf, jobName); ok {
		s.emitJobNotification(ctx, wf, run, job.Spec, model.NotifyEventPassed)
	}
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if !runAllowsProgress(run) {
		return nil
	}
	stages := WorkflowStagesForRun(wf, run)
	if s.stageComplete(run, run.CurrentStage) {
		return s.advanceStage(ctx, wf, run)
	}
	if run.CurrentStage < len(stages) && !stages[run.CurrentStage].Parallel {
		return s.executeCurrentStage(ctx, wf, run)
	}
	return nil
}

func (s *Scheduler) HandleDeployCompleted(ctx context.Context, deploymentID kernel.DeploymentRunID, status string) error {
	ref, err := s.repo.FindRunByDeploymentID(ctx, deploymentID)
	if err != nil {
		return nil
	}
	unlock := s.locker.lock(ref.RunID)
	defer unlock()
	return s.handleDeployCompletedLocked(ctx, ref.RunID, ref.JobName, deploymentID, status)
}

func (s *Scheduler) handleDeployCompletedLocked(
	ctx context.Context,
	runID kernel.ProductWorkflowRunID,
	jobName string,
	deploymentID kernel.DeploymentRunID,
	status string,
) error {
	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusRunning && run.Status != model.RunStatusPaused {
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
			if jobRun.Name != jobName {
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
	if !runAllowsProgress(run) {
		return nil
	}
	stages := WorkflowStagesForRun(wf, run)
	if s.stageComplete(run, run.CurrentStage) {
		return s.advanceStage(ctx, wf, run)
	}
	if run.CurrentStage < len(stages) && !stages[run.CurrentStage].Parallel {
		return s.executeCurrentStage(ctx, wf, run)
	}
	return nil
}

func (s *Scheduler) syncExternalStatusesOnRun(ctx context.Context, run *model.ProductWorkflowRun) bool {
	if s == nil || s.status == nil || run == nil {
		return false
	}
	changed := false
	for si := range run.StageRuns {
		for ji := range run.StageRuns[si].Jobs {
			job := &run.StageRuns[si].Jobs[ji]
			for bi := range job.Builds {
				build := &job.Builds[bi]
				if build.BuildID == "" || jobRunTerminal(build.Status) {
					continue
				}
				buildID, err := kernel.ParseBuildID(build.BuildID)
				if err != nil {
					continue
				}
				status, terminal, err := s.status.BuildStatus(ctx, buildID)
				if err != nil || !terminal {
					continue
				}
				switch status {
				case string(buildmodel.BuildStatusSuccess):
					build.Status = model.JobRunStatusSuccess
					changed = true
				case string(buildmodel.BuildStatusFailed), string(buildmodel.BuildStatusAborted):
					build.Status = model.JobRunStatusFailed
					if build.ErrorMessage == "" {
						build.ErrorMessage = "build " + status
					}
					changed = true
				}
			}
			for di := range job.Deploys {
				deploy := &job.Deploys[di]
				if deploy.DeploymentRunID == "" || jobRunTerminal(deploy.Status) {
					continue
				}
				deploymentID, err := kernel.ParseDeploymentRunID(deploy.DeploymentRunID)
				if err != nil {
					continue
				}
				status, terminal, err := s.status.DeployStatus(ctx, deploymentID)
				if err != nil || !terminal {
					continue
				}
				switch status {
				case string(deployrunmodel.StatusSucceeded):
					deploy.Status = model.JobRunStatusSuccess
					changed = true
				case string(deployrunmodel.StatusFailed), string(deployrunmodel.StatusAborted):
					deploy.Status = model.JobRunStatusFailed
					if deploy.ErrorMessage == "" {
						deploy.ErrorMessage = "deployment " + status
					}
					changed = true
				}
			}
		}
	}
	return changed
}

// SyncRunExternalStatuses refreshes terminal build/deploy statuses for UI reads.
// It never advances the workflow or triggers new jobs, and skips work when the run lock is busy.
func (s *Scheduler) SyncRunExternalStatuses(ctx context.Context, runID kernel.ProductWorkflowRunID) error {
	if s == nil || s.status == nil {
		return nil
	}
	unlock, ok := s.locker.tryLock(runID)
	if !ok {
		return nil
	}
	defer unlock()

	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if !runNeedsReconcile(run) {
		return nil
	}
	if !s.syncExternalStatusesOnRun(ctx, run) {
		return nil
	}
	return s.repo.UpdateRun(ctx, run)
}

// ProgressStuckRun syncs external statuses, refreshes job states, and advances or executes pending work.
func (s *Scheduler) ProgressStuckRun(ctx context.Context, runID kernel.ProductWorkflowRunID) error {
	if s == nil {
		return nil
	}
	unlock, ok := s.locker.tryLock(runID)
	if !ok {
		return nil
	}
	defer unlock()

	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if !runNeedsReconcile(run) {
		return nil
	}
	wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return err
	}

	_ = s.syncExternalStatusesOnRun(ctx, run)
	if failed, msg := s.refreshJobStatuses(run); failed {
		return s.failRun(ctx, run, msg)
	}
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if !runAllowsProgress(run) {
		return nil
	}
	stages := WorkflowStagesForRun(wf, run)
	if s.stageComplete(run, run.CurrentStage) {
		return s.advanceStage(ctx, wf, run)
	}
	if run.CurrentStage < len(stages) && run.CurrentStage < len(run.StageRuns) {
		return s.executeCurrentStage(ctx, wf, run)
	}
	return s.healStuckRun(ctx, run)
}

func (s *Scheduler) ReconcileRun(ctx context.Context, runID kernel.ProductWorkflowRunID) error {
	if s == nil || s.status == nil {
		return nil
	}
	unlock, ok := s.locker.tryLock(runID)
	if !ok {
		return nil
	}
	defer unlock()

	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if !runNeedsReconcile(run) {
		return nil
	}

	for si := range run.StageRuns {
		for ji := range run.StageRuns[si].Jobs {
			job := &run.StageRuns[si].Jobs[ji]
			for _, build := range job.Builds {
				if build.BuildID == "" || jobRunTerminal(build.Status) {
					continue
				}
				buildID, err := kernel.ParseBuildID(build.BuildID)
				if err != nil {
					continue
				}
				status, terminal, err := s.status.BuildStatus(ctx, buildID)
				if err != nil || !terminal {
					continue
				}
				if err := s.handleBuildCompletedLocked(ctx, runID, job.Name, buildID, status); err != nil {
					return err
				}
				run, err = s.repo.FindRunByID(ctx, runID)
				if err != nil || !runNeedsReconcile(run) {
					return err
				}
			}
			for _, deploy := range job.Deploys {
				if deploy.DeploymentRunID == "" || jobRunTerminal(deploy.Status) {
					continue
				}
				deploymentID, err := kernel.ParseDeploymentRunID(deploy.DeploymentRunID)
				if err != nil {
					continue
				}
				status, terminal, err := s.status.DeployStatus(ctx, deploymentID)
				if err != nil || !terminal {
					continue
				}
				if err := s.handleDeployCompletedLocked(ctx, runID, job.Name, deploymentID, status); err != nil {
					return err
				}
				run, err = s.repo.FindRunByID(ctx, runID)
				if err != nil || !runNeedsReconcile(run) {
					return err
				}
			}
		}
	}
	run, err = s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	return s.healStuckRun(ctx, run)
}

func (s *Scheduler) healStuckRun(ctx context.Context, run *model.ProductWorkflowRun) error {
	if run == nil || !runNeedsReconcile(run) || run.Status == model.RunStatusPaused {
		return nil
	}
	allDone := true
	for si := range run.StageRuns {
		if !s.stageComplete(run, si) {
			allDone = false
			break
		}
	}
	if !allDone {
		return nil
	}
	wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return err
	}
	stages := WorkflowStagesForRun(wf, run)
	if run.CurrentStage >= len(stages) || run.CurrentStage >= len(run.StageRuns) {
		return s.completeRun(ctx, run)
	}
	return s.advanceStage(ctx, wf, run)
}

func runNeedsReconcile(run *model.ProductWorkflowRun) bool {
	if run == nil {
		return false
	}
	switch run.Status {
	case model.RunStatusPending, model.RunStatusRunning, model.RunStatusPaused:
		return true
	default:
		return false
	}
}

func jobRunTerminal(status model.JobRunStatus) bool {
	switch status {
	case model.JobRunStatusSuccess, model.JobRunStatusFailed, model.JobRunStatusSkipped:
		return true
	default:
		return false
	}
}

func (s *Scheduler) ApproveJob(ctx context.Context, runID kernel.ProductWorkflowRunID, jobName string, actor kernel.MemberID) error {
	run, wf, jobRun, err := s.loadPendingApproval(ctx, runID, jobName)
	if err != nil {
		return err
	}
	job, ok := jobByName(wf, jobName)
	if !ok {
		return domain.ErrApprovalNotPending
	}
	spec := ParseApprovalSpec(job.Spec)
	actorID := actor.String()
	if len(spec.ApproverIDs) > 0 && !containsString(spec.ApproverIDs, actorID) {
		return domain.ErrApprovalForbidden
	}
	for _, approvedBy := range jobRun.Approval.ApprovedBy {
		if approvedBy == actorID {
			return domain.ErrApprovalForbidden
		}
	}
	jobRun.Approval.ApprovedBy = append(jobRun.Approval.ApprovedBy, actorID)
	jobRun.Approval.ActorID = actorID
	required := jobRun.Approval.RequiredApprovals
	if required <= 0 {
		required = spec.RequiredApprovals
	}
	if required <= 0 {
		required = 1
	}
	if len(jobRun.Approval.ApprovedBy) < required {
		if err := s.repo.UpdateRun(ctx, run); err != nil {
			return err
		}
		return nil
	}
	jobRun.Approval.Status = model.ApprovalStatusApproved
	jobRun.Status = model.JobRunStatusSuccess
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.emitJobNotification(ctx, wf, run, job.Spec, model.NotifyEventPassed)
	stages := WorkflowStagesForRun(wf, run)
	if s.stageComplete(run, run.CurrentStage) {
		return s.advanceStage(ctx, wf, run)
	}
	if run.CurrentStage < len(stages) && !stages[run.CurrentStage].Parallel {
		return s.executeCurrentStage(ctx, wf, run)
	}
	return nil
}

func (s *Scheduler) CancelRun(ctx context.Context, runID kernel.ProductWorkflowRunID, message string) error {
	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	switch run.Status {
	case model.RunStatusSuccess, model.RunStatusFailed:
		return nil
	case model.RunStatusPending, model.RunStatusRunning, model.RunStatusPaused:
	default:
		return domain.ErrInvalidWorkflowInput
	}
	if strings.TrimSpace(message) == "" {
		message = "用户终止"
	}
	for si := range run.StageRuns {
		stage := &run.StageRuns[si]
		switch stage.Status {
		case model.JobRunStatusSuccess, model.JobRunStatusSkipped:
		default:
			stage.Status = model.JobRunStatusFailed
		}
		for ji := range stage.Jobs {
			job := &stage.Jobs[ji]
			switch job.Status {
			case model.JobRunStatusSuccess, model.JobRunStatusSkipped:
				continue
			default:
				job.Status = model.JobRunStatusFailed
				if job.ErrorMessage == "" {
					job.ErrorMessage = message
				}
			}
			for bi := range job.Builds {
				if job.Builds[bi].Status != model.JobRunStatusSuccess {
					job.Builds[bi].Status = model.JobRunStatusFailed
				}
			}
			for di := range job.Deploys {
				if job.Deploys[di].Status != model.JobRunStatusSuccess {
					job.Deploys[di].Status = model.JobRunStatusFailed
					if job.Deploys[di].ErrorMessage == "" {
						job.Deploys[di].ErrorMessage = message
					}
				}
			}
		}
	}
	return s.failRunEvent(ctx, run, message, model.NotifyEventCancelled)
}

func (s *Scheduler) RejectJob(ctx context.Context, runID kernel.ProductWorkflowRunID, jobName string, actor kernel.MemberID, message string) error {
	run, wf, jobRun, err := s.loadPendingApproval(ctx, runID, jobName)
	if err != nil {
		return err
	}
	if job, ok := jobByName(wf, jobName); ok {
		spec := ParseApprovalSpec(job.Spec)
		actorID := actor.String()
		if len(spec.ApproverIDs) > 0 && !containsString(spec.ApproverIDs, actorID) {
			return domain.ErrApprovalForbidden
		}
	}
	jobRun.Approval.Status = model.ApprovalStatusRejected
	jobRun.Approval.ActorID = actor.String()
	jobRun.Approval.Message = message
	jobRun.Status = model.JobRunStatusFailed
	jobRun.ErrorMessage = message
	return s.failRunEvent(ctx, run, "approval rejected", model.NotifyEventRejected)
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
	stages := WorkflowStagesForRun(wf, run)
	if run.CurrentStage >= len(stages) || run.CurrentStage >= len(run.StageRuns) {
		return s.completeRun(ctx, run)
	}
	stage := stages[run.CurrentStage]
	stageRun := &run.StageRuns[run.CurrentStage]
	stageRun.ExecMode = model.NormalizeStageExecMode(stage.ExecMode)
	if (stageRun.ExecMode == model.StageExecManual || (wf != nil && wf.Advanced.Debug)) && !stageRun.ManualConfirmed {
		stageRun.Status = model.JobRunStatusPaused
		run.Status = model.RunStatusPaused
		if err := s.repo.UpdateRun(ctx, run); err != nil {
			return err
		}
		s.emitNotification(ctx, wf, run, model.NotifyEventManual)
		return nil
	}
	stageRun.Status = model.JobRunStatusRunning
	if run.Status == model.RunStatusPaused {
		run.Status = model.RunStatusRunning
	}

	hasAsync := false
	for ji, job := range stage.Jobs {
		jobRun := &stageRun.Jobs[ji]
		if jobRun.Status == model.JobRunStatusSuccess || jobRun.Status == model.JobRunStatusSkipped {
			continue
		}
		if jobRun.Status == model.JobRunStatusRunning || IsApprovalWaiting(*jobRun) {
			hasAsync = true
			if !stage.Parallel {
				break
			}
			continue
		}
		switch job.Type {
		case model.JobTypeSparkopsBuild:
			hasAsync = true
			if err := s.builds.RunJob(ctx, run, run.CurrentStage, ji, job); err != nil {
				return s.failRun(ctx, run, jobRun.ErrorMessage)
			}
			s.emitJobNotification(ctx, wf, run, job.Spec, model.NotifyEventCreated)
		case model.JobTypeSparkopsCreateVersion:
			if s.createVersions == nil {
				jobRun.Status = model.JobRunStatusFailed
				jobRun.ErrorMessage = "version creator unavailable"
				return s.failRun(ctx, run, jobRun.ErrorMessage)
			}
			if err := s.createVersions.RunJob(ctx, wf, run, run.CurrentStage, ji, job); err != nil {
				return s.failRun(ctx, run, jobRun.ErrorMessage)
			}
		case model.JobTypeSparkopsDeploy, model.JobTypeSparkopsDeployVersion:
			hasAsync = true
			if s.deploys == nil {
				jobRun.Status = model.JobRunStatusFailed
				jobRun.ErrorMessage = "deploy executor unavailable"
				return s.failRun(ctx, run, jobRun.ErrorMessage)
			}
			if err := s.deploys.RunJob(ctx, wf, run, run.CurrentStage, ji, job); err != nil {
				return s.failRun(ctx, run, jobRun.ErrorMessage)
			}
		case model.JobTypeSparkopsApproval:
			hasAsync = true
			StartApprovalJob(jobRun, job)
			s.emitNotification(ctx, wf, run, model.NotifyEventWaiting)
		case model.JobTypeFreestyle:
			RunFreestyleJob(jobRun, job)
		case model.JobTypeNotify:
			RunNotifyJob(jobRun, job)
		default:
			jobRun.Status = model.JobRunStatusSkipped
		}
		if !stage.Parallel && jobRun.Status != model.JobRunStatusSuccess && jobRun.Status != model.JobRunStatusSkipped {
			break
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

func jobByName(wf *model.ProductWorkflow, name string) (model.WorkflowJob, bool) {
	if wf == nil {
		return model.WorkflowJob{}, false
	}
	for _, stage := range wf.Stages {
		for _, job := range stage.Jobs {
			if job.Name == name {
				return job, true
			}
		}
	}
	return model.WorkflowJob{}, false
}

func StageNeedsManualConfirm(stage model.WorkflowStage, stageRun model.StageRun) bool {
	return model.NormalizeStageExecMode(stage.ExecMode) == model.StageExecManual && !stageRun.ManualConfirmed
}

func (s *Scheduler) ContinueManualStage(ctx context.Context, runID kernel.ProductWorkflowRunID) error {
	run, err := s.repo.FindRunByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != model.RunStatusPaused || run.CurrentStage >= len(run.StageRuns) {
		return domain.ErrStageNotWaitingManual
	}
	stageRun := &run.StageRuns[run.CurrentStage]
	if stageRun.Status != model.JobRunStatusPaused {
		return domain.ErrStageNotWaitingManual
	}
	wf, err := s.repo.FindWorkflowByID(ctx, run.WorkflowID)
	if err != nil {
		return err
	}
	stageRun.ManualConfirmed = true
	run.Status = model.RunStatusRunning
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	return s.executeCurrentStage(ctx, wf, run)
}

func (s *Scheduler) advanceStage(ctx context.Context, wf *model.ProductWorkflow, run *model.ProductWorkflowRun) error {
	stages := WorkflowStagesForRun(wf, run)
	run.StageRuns[run.CurrentStage].Status = model.JobRunStatusSuccess
	run.CurrentStage++
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if run.CurrentStage >= len(stages) || run.CurrentStage >= len(run.StageRuns) {
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
			case model.JobTypeSparkopsDeploy, model.JobTypeSparkopsDeployVersion:
				if failed, msg := refreshDeployJob(jobRun); failed {
					run.StageRuns[si].Status = model.JobRunStatusFailed
					return true, msg
				}
			case model.JobTypeSparkopsApproval:
				if IsApprovalWaiting(*jobRun) {
					if approvalTimedOut(jobRun.Approval) {
						jobRun.Approval.Status = model.ApprovalStatusRejected
						jobRun.Status = model.JobRunStatusFailed
						jobRun.ErrorMessage = "approval timeout"
						run.StageRuns[si].Status = model.JobRunStatusFailed
						return true, "approval timeout"
					}
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
	s.emitNotification(ctx, nil, run, model.NotifyEventPassed)
	s.reportGit(ctx, run)
	return s.publishRunCompleted(ctx, run)
}

func (s *Scheduler) failRun(ctx context.Context, run *model.ProductWorkflowRun, message string) error {
	return s.failRunEvent(ctx, run, message, model.NotifyEventFailed)
}

func (s *Scheduler) failRunEvent(ctx context.Context, run *model.ProductWorkflowRun, message, event string) error {
	now := time.Now().UTC()
	run.Status = model.RunStatusFailed
	if run.ErrorMessage == "" {
		run.ErrorMessage = message
	}
	run.CompletedAt = &now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.emitNotification(ctx, nil, run, event)
	s.reportGit(ctx, run)
	return s.publishRunCompleted(ctx, run)
}

func (s *Scheduler) ExpireRun(ctx context.Context, run *model.ProductWorkflowRun) error {
	if run == nil || run.Status == model.RunStatusSuccess || run.Status == model.RunStatusFailed {
		return nil
	}
	return s.failRunEvent(ctx, run, "工作流超时", model.NotifyEventTimeout)
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

// ApplyRunInput overlays runtime branch / environment overrides onto a copy of stages.
func ApplyRunInput(stages []model.WorkflowStage, input map[string]any) []model.WorkflowStage {
	if len(input) == 0 {
		return stages
	}
	out := cloneStages(stages)
	branch, _ := input["branch"].(string)
	envOverrides, _ := input["environment_overrides"].(map[string]any)
	for si := range out {
		for ji := range out[si].Jobs {
			job := &out[si].Jobs[ji]
			if job.Spec == nil {
				job.Spec = map[string]any{}
			}
			if branch != "" && job.Type == model.JobTypeSparkopsBuild {
				job.Spec["branch"] = branch
				if builds, ok := job.Spec["project_builds"].([]any); ok {
					for bi := range builds {
						if m, ok := builds[bi].(map[string]any); ok {
							m["branch"] = branch
							builds[bi] = m
						}
					}
					job.Spec["project_builds"] = builds
				}
			}
			if job.Type == model.JobTypeSparkopsBuild {
				applyBranchOverrides(job, stringMap(input["branch_overrides"]), stringMap(input["ref_type_overrides"]))
			}
			if (job.Type == model.JobTypeSparkopsDeploy || job.Type == model.JobTypeSparkopsDeployVersion) && len(envOverrides) > 0 {
				if envID, ok := envOverrides[job.Name].(string); ok && envID != "" {
					job.Spec["environment_id"] = envID
				}
			}
			if job.Type == model.JobTypeSparkopsCreateVersion {
				if tagOverrides, ok := input["version_tag_overrides"].(map[string]any); ok {
					if tag, ok := tagOverrides[job.Name].(string); ok && strings.TrimSpace(tag) != "" {
						job.Spec["version_tag"] = strings.TrimSpace(tag)
					}
				}
			}
			if (job.Type == model.JobTypeSparkopsDeploy || job.Type == model.JobTypeSparkopsDeployVersion) &&
				deployArtifactSource(job.Type, job.Spec) == "version" {
				if tagOverrides, ok := input["version_tag_overrides"].(map[string]any); ok {
					if tag, ok := tagOverrides[job.Name].(string); ok && strings.TrimSpace(tag) != "" {
						job.Spec["version_tag"] = strings.TrimSpace(tag)
					}
				}
			}
		}
	}
	return out
}

func applyBranchOverrides(job *model.WorkflowJob, overrides, refTypes map[string]string) {
	if len(overrides) == 0 && len(refTypes) == 0 {
		return
	}
	builds, ok := job.Spec["project_builds"].([]any)
	if !ok || len(builds) == 0 {
		if branch := overrides[job.Name]; branch != "" {
			job.Spec["branch"] = branch
		}
		if refType := refTypes[job.Name]; refType != "" {
			job.Spec["git_ref_type"] = refType
		}
		return
	}
	for i := range builds {
		row, ok := builds[i].(map[string]any)
		if !ok {
			continue
		}
		projectID, _ := row["project_id"].(string)
		key := job.Name + ":" + projectID
		branch := overrides[key]
		if branch == "" {
			branch = overrides[projectID]
		}
		if branch != "" {
			row["branch"] = branch
		}
		refType := refTypes[key]
		if refType == "" {
			refType = refTypes[projectID]
		}
		if refType != "" {
			row["git_ref_type"] = refType
		}
		builds[i] = row
	}
	job.Spec["project_builds"] = builds
	if first, ok := builds[0].(map[string]any); ok {
		if branch, _ := first["branch"].(string); branch != "" {
			job.Spec["branch"] = branch
		}
		if refType, _ := first["git_ref_type"].(string); refType != "" {
			job.Spec["git_ref_type"] = refType
		}
	}
}

func cloneStages(stages []model.WorkflowStage) []model.WorkflowStage {
	raw, err := json.Marshal(stages)
	if err != nil {
		return stages
	}
	var out []model.WorkflowStage
	if err := json.Unmarshal(raw, &out); err != nil {
		return stages
	}
	return out
}

func ValidateStages(stages []model.WorkflowStage) error {
	if len(stages) == 0 {
		return domain.ErrInvalidWorkflowInput
	}
	seenJobs := map[string]struct{}{}
	for _, stage := range stages {
		if strings.TrimSpace(stage.Name) == "" {
			return domain.ErrInvalidWorkflowInput
		}
		for _, job := range stage.Jobs {
			name := strings.TrimSpace(job.Name)
			if name == "" || strings.TrimSpace(job.Type) == "" {
				return domain.ErrInvalidWorkflowInput
			}
			if _, ok := seenJobs[name]; ok {
				return domain.ErrInvalidWorkflowInput
			}
			seenJobs[name] = struct{}{}
			if !model.IsKnownJobType(job.Type) {
				return domain.ErrInvalidWorkflowInput
			}
		}
	}
	return validateStageJobRefs(stages)
}
