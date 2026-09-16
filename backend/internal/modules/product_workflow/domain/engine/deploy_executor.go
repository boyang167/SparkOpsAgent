package engine

import (
	"context"
	"fmt"
	"strings"

	deploymodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DeployExecutor struct {
	trigger appctx.DeploymentTrigger
	repo    repository.ProductWorkflowRepository
}

func NewDeployExecutor(trigger appctx.DeploymentTrigger, repo repository.ProductWorkflowRepository) *DeployExecutor {
	return &DeployExecutor{trigger: trigger, repo: repo}
}

func (e *DeployExecutor) RunJob(
	ctx context.Context,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
) error {
	if e.trigger == nil {
		return fmt.Errorf("deployment trigger unavailable")
	}
	jobRun := &run.StageRuns[stageIndex].Jobs[jobIndex]
	jobRun.Type = job.Type
	spec := job.Spec
	if spec == nil {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "missing deploy spec"
		return fmt.Errorf("missing deploy spec")
	}
	envRaw, _ := spec["environment_id"].(string)
	envID, err := kernel.ParseEnvironmentID(strings.TrimSpace(envRaw))
	if err != nil {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "invalid environment_id"
		return err
	}
	source, _ := spec["source"].(string)
	if source != "fromjob" {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "only source=fromjob is supported in M6"
		return fmt.Errorf("unsupported deploy source")
	}
	refJobName, _ := spec["job_name"].(string)
	targets := resolveFromJobBuilds(run, refJobName)
	if len(targets) == 0 {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "no successful builds from job " + refJobName
		return fmt.Errorf("no build outputs")
	}

	jobRun.Deploys = make([]model.ProjectDeployRun, 0, len(targets))
	jobRun.Status = model.JobRunStatusRunning
	run.StageRuns[stageIndex].Status = model.JobRunStatusRunning

	for _, target := range targets {
		projectID, parseErr := kernel.ParseProjectID(target.ProjectID)
		if parseErr != nil {
			return parseErr
		}
		branch := target.Branch
		if branch == "" {
			branch = "main"
		}
		imageTag := deploymodel.DefaultImageTag(branch)
		deployID, triggerErr := e.trigger.TriggerForEnvironment(ctx, appctx.WorkflowDeploymentRequest{
			ProjectID: projectID, EnvironmentID: envID,
			GitRefType: "branch", GitRefValue: branch, ImageTag: imageTag,
		})
		if triggerErr != nil {
			jobRun.Status = model.JobRunStatusFailed
			jobRun.ErrorMessage = triggerErr.Error()
			jobRun.Deploys = append(jobRun.Deploys, model.ProjectDeployRun{
				ProjectID: target.ProjectID, EnvironmentID: envID.String(), ImageTag: imageTag,
				Status: model.JobRunStatusFailed, ErrorMessage: triggerErr.Error(),
			})
			return triggerErr
		}
		jobRun.Deploys = append(jobRun.Deploys, model.ProjectDeployRun{
			ProjectID: target.ProjectID, EnvironmentID: envID.String(),
			DeploymentRunID: deployID.String(), ImageTag: imageTag, Status: model.JobRunStatusRunning,
		})
		if err := e.repo.SaveDeployRef(ctx, &model.WorkflowDeployRef{
			DeploymentRunID: deployID, RunID: run.ID, JobName: job.Name, ProjectID: projectID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func resolveFromJobBuilds(run *model.ProductWorkflowRun, jobName string) []model.ProjectBuildRun {
	out := make([]model.ProjectBuildRun, 0)
	for _, stage := range run.StageRuns {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			for _, b := range job.Builds {
				if b.Status == model.JobRunStatusSuccess {
					out = append(out, b)
				}
			}
		}
	}
	return out
}
