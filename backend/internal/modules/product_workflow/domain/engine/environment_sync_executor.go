package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type EnvironmentSyncExecutor struct {
	snapshots appctx.EnvironmentSyncSnapshots
	trigger   appctx.EnvironmentSyncTrigger
	repo      repository.ProductWorkflowRepository
}

func NewEnvironmentSyncExecutor(
	snapshots appctx.EnvironmentSyncSnapshots,
	trigger appctx.EnvironmentSyncTrigger,
	repo repository.ProductWorkflowRepository,
) *EnvironmentSyncExecutor {
	return &EnvironmentSyncExecutor{snapshots: snapshots, trigger: trigger, repo: repo}
}

func (e *EnvironmentSyncExecutor) RunJob(
	ctx context.Context,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
) error {
	jobRun := &run.StageRuns[stageIndex].Jobs[jobIndex]
	jobRun.Type = job.Type
	if e.snapshots == nil || e.trigger == nil {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "environment sync unavailable"
		return fmt.Errorf("environment sync unavailable")
	}
	snapshotID, err := environmentSyncPreviewID(run, job.Name)
	if err != nil {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = err.Error()
		return err
	}
	snapshot, err := e.snapshots.LoadBound(ctx, snapshotID, run.ID)
	if err != nil {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = err.Error()
		return err
	}

	prior := reusableDeploys(jobRun.Deploys)
	jobRun.Deploys = make([]model.ProjectDeployRun, 0, len(snapshot.Projects))
	jobRun.Status = model.JobRunStatusRunning
	run.StageRuns[stageIndex].Status = model.JobRunStatusRunning

	targetEnv := snapshot.TargetEnvironmentID.String()
	sourceEnv := snapshot.SourceEnvironmentID.String()
	for _, project := range snapshot.Projects {
		projectID := project.ProjectID.String()
		if kept, ok := prior[projectID]; ok {
			jobRun.Deploys = append(jobRun.Deploys, kept)
			continue
		}
		deployID, triggerErr := e.trigger.TriggerEnvironmentSync(ctx, snapshot.ID, project.ProjectID)
		row := model.ProjectDeployRun{
			ProjectID:           projectID,
			EnvironmentID:       targetEnv,
			SourceEnvironmentID: sourceEnv,
			SyncImage:           project.Option.SyncImage,
			SyncYAML:            project.Option.SyncYAML,
			FirstSync:           project.Option.FirstSync,
			Status:              model.JobRunStatusRunning,
		}
		if triggerErr != nil {
			row.Status = model.JobRunStatusFailed
			row.ErrorMessage = triggerErr.Error()
			jobRun.Deploys = append(jobRun.Deploys, row)
			continue
		}
		row.DeploymentRunID = deployID.String()
		jobRun.Deploys = append(jobRun.Deploys, row)
		if err := e.repo.SaveDeployRef(ctx, &model.WorkflowDeployRef{
			DeploymentRunID: deployID,
			RunID:           run.ID,
			JobName:         job.Name,
			ProjectID:       project.ProjectID,
		}); err != nil {
			return err
		}
	}
	refreshEnvironmentSyncDeployJob(jobRun)
	return nil
}

func environmentSyncPreviewID(run *model.ProductWorkflowRun, jobName string) (kernel.EnvironmentSyncSnapshotID, error) {
	if run == nil || len(run.Input) == 0 {
		return kernel.EnvironmentSyncSnapshotID{}, fmt.Errorf("missing environment sync preview")
	}
	var input map[string]any
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return kernel.EnvironmentSyncSnapshotID{}, fmt.Errorf("missing environment sync preview")
	}
	previewIDs, _ := input["environment_sync_preview_ids"].(map[string]any)
	rawID, _ := previewIDs[jobName].(string)
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return kernel.EnvironmentSyncSnapshotID{}, fmt.Errorf("missing environment sync preview for job %s", jobName)
	}
	return kernel.ParseEnvironmentSyncSnapshotID(rawID)
}

func refreshEnvironmentSyncDeployJob(jobRun *model.JobRun) (bool, string) {
	if jobRun == nil || len(jobRun.Deploys) == 0 {
		return false, ""
	}
	allTerminal := true
	allSuccess := true
	anyFailed := false
	for _, deploy := range jobRun.Deploys {
		switch deploy.Status {
		case model.JobRunStatusFailed:
			anyFailed = true
			allSuccess = false
		case model.JobRunStatusRunning, model.JobRunStatusPending:
			allTerminal = false
			allSuccess = false
		case model.JobRunStatusSuccess:
		default:
			allTerminal = false
			allSuccess = false
		}
	}
	if !allTerminal {
		jobRun.Status = model.JobRunStatusRunning
		return false, ""
	}
	if anyFailed {
		jobRun.Status = model.JobRunStatusFailed
		return true, "environment sync job failed"
	}
	if allSuccess {
		jobRun.Status = model.JobRunStatusSuccess
	}
	return false, ""
}
