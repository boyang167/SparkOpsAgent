package engine

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BuildExecutor struct {
	trigger appctx.BuildTrigger
	repo    repository.ProductWorkflowRepository
}

func NewBuildExecutor(trigger appctx.BuildTrigger, repo repository.ProductWorkflowRepository) *BuildExecutor {
	return &BuildExecutor{trigger: trigger, repo: repo}
}

func (e *BuildExecutor) RunJob(
	ctx context.Context,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
) error {
	jobRun := &run.StageRuns[stageIndex].Jobs[jobIndex]
	jobRun.Type = job.Type
	builds := parseProjectBuilds(job.Spec)
	if len(builds) == 0 {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "sparkops-build requires project_builds"
		return fmt.Errorf("missing project_builds")
	}
	jobRun.Builds = make([]model.ProjectBuildRun, 0, len(builds))
	jobRun.Status = model.JobRunStatusRunning
	run.StageRuns[stageIndex].Status = model.JobRunStatusRunning

	for _, spec := range builds {
		projectID, err := kernel.ParseProjectID(spec.ProjectID)
		if err != nil {
			return err
		}
		branch := spec.Branch
		if branch == "" {
			branch = "main"
		}
		buildID, err := e.trigger.TriggerAutomated(ctx, projectID, branch)
		if err != nil {
			jobRun.Status = model.JobRunStatusFailed
			jobRun.ErrorMessage = err.Error()
			jobRun.Builds = append(jobRun.Builds, model.ProjectBuildRun{
				ProjectID: spec.ProjectID, Branch: branch, Status: model.JobRunStatusFailed, ErrorMessage: err.Error(),
			})
			return err
		}
		jobRun.Builds = append(jobRun.Builds, model.ProjectBuildRun{
			ProjectID: spec.ProjectID, Branch: branch, BuildID: buildID.String(), Status: model.JobRunStatusRunning,
		})
		if err := e.repo.SaveBuildRef(ctx, &model.WorkflowBuildRef{
			BuildID: buildID, RunID: run.ID, JobName: job.Name, ProjectID: projectID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func parseProjectBuilds(spec map[string]any) []model.ProjectBuildSpec {
	raw, ok := spec["project_builds"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]model.ProjectBuildSpec, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pid, _ := m["project_id"].(string)
		branch, _ := m["branch"].(string)
		if pid == "" {
			continue
		}
		out = append(out, model.ProjectBuildSpec{ProjectID: pid, Branch: branch})
	}
	return out
}
