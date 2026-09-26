package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateVersionExecutor struct {
	versions appctx.VersionCreator
}

func NewCreateVersionExecutor(versions appctx.VersionCreator) *CreateVersionExecutor {
	return &CreateVersionExecutor{versions: versions}
}

func (e *CreateVersionExecutor) RunJob(
	ctx context.Context,
	wf *model.ProductWorkflow,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
) error {
	if e.versions == nil {
		return fmt.Errorf("version creator unavailable")
	}
	jobRun := &run.StageRuns[stageIndex].Jobs[jobIndex]
	jobRun.Type = job.Type
	spec := job.Spec
	if spec == nil {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "missing create-version spec"
		return fmt.Errorf("missing create-version spec")
	}
	versionTag, _ := spec["version_tag"].(string)
	versionTag = strings.TrimSpace(versionTag)
	if versionTag == "" {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "version_tag is required"
		return fmt.Errorf("version_tag is required")
	}
	source, _ := spec["source"].(string)
	if source != "fromjob" {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "only source=fromjob is supported"
		return fmt.Errorf("unsupported create-version source")
	}
	refJobName, _ := spec["job_name"].(string)
	targets := resolveFromJobBuilds(run, refJobName)
	if len(targets) == 0 {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "no successful builds from job " + refJobName
		return fmt.Errorf("no build outputs")
	}
	allowed := parseProjectIDFilter(spec["project_ids"])
	jobRun.Versions = make([]model.ProjectVersionRun, 0, len(targets))
	for _, target := range targets {
		if len(allowed) > 0 {
			if _, ok := allowed[target.ProjectID]; !ok {
				continue
			}
		}
		projectID, err := kernel.ParseProjectID(target.ProjectID)
		if err != nil {
			return err
		}
		branch := target.Branch
		if branch == "" {
			branch = "main"
		}
		gitRefType, imageTag, imageRepository := deployImage(target, branch)
		versionID, err := e.versions.CreateFromWorkflow(ctx, appctx.CreateVersionRequest{
			WorkspaceID: wf.WorkspaceID, ProjectID: projectID, ProductID: run.ProductID,
			VersionTag: versionTag, WorkflowRunID: run.ID, WorkflowJobName: job.Name,
			GitRefType: gitRefType, GitRefValue: branch,
			ImageTag: imageTag, ImageRepository: imageRepository, BuildID: target.BuildID,
			CreatedBy: wf.CreatedBy,
		})
		if err != nil {
			jobRun.Status = model.JobRunStatusFailed
			jobRun.ErrorMessage = err.Error()
			jobRun.Versions = append(jobRun.Versions, model.ProjectVersionRun{
				ProjectID: target.ProjectID, VersionTag: versionTag,
				Status: model.JobRunStatusFailed, ErrorMessage: err.Error(),
			})
			return err
		}
		jobRun.Versions = append(jobRun.Versions, model.ProjectVersionRun{
			ProjectID: target.ProjectID, VersionID: versionID.String(),
			VersionTag: versionTag, Status: model.JobRunStatusSuccess,
		})
	}
	if len(jobRun.Versions) == 0 {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "no projects matched for version creation"
		return fmt.Errorf("no projects matched")
	}
	jobRun.Status = model.JobRunStatusSuccess
	return nil
}

func parseProjectIDFilter(raw any) map[string]struct{} {
	out := map[string]struct{}{}
	items, ok := raw.([]any)
	if !ok {
		return out
	}
	for _, item := range items {
		id, _ := item.(string)
		id = strings.TrimSpace(id)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

func resolveFromJobVersions(run *model.ProductWorkflowRun, jobName string) []model.ProjectVersionRun {
	out := make([]model.ProjectVersionRun, 0)
	for _, stage := range run.StageRuns {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			for _, version := range job.Versions {
				if version.Status == model.JobRunStatusSuccess {
					out = append(out, version)
				}
			}
		}
	}
	return out
}
