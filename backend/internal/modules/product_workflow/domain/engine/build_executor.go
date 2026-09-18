package engine

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	deploymodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ImageRepositoryResolver interface {
	Resolve(ctx context.Context, projectID kernel.ProjectID, registrySecretID string) (string, error)
}

type BuildExecutor struct {
	trigger  appctx.BuildTrigger
	repo     repository.ProductWorkflowRepository
	registry ImageRepositoryResolver
}

func NewBuildExecutor(trigger appctx.BuildTrigger, repo repository.ProductWorkflowRepository, registry ImageRepositoryResolver) *BuildExecutor {
	return &BuildExecutor{trigger: trigger, repo: repo, registry: registry}
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

	registryID := parseRegistryID(job.Spec)
	if registryID != "" && e.registry == nil {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "image registry resolver unavailable"
		return fmt.Errorf("image registry resolver unavailable")
	}

	for _, spec := range builds {
		projectID, err := kernel.ParseProjectID(spec.ProjectID)
		if err != nil {
			return err
		}
		branch := spec.Branch
		if branch == "" {
			branch = "main"
		}
		repoPath := ""
		if registryID != "" {
			resolved, err := e.registry.Resolve(ctx, projectID, registryID)
			if err != nil {
				jobRun.Status = model.JobRunStatusFailed
				jobRun.ErrorMessage = err.Error()
				jobRun.Builds = append(jobRun.Builds, failedBuildRun(spec, branch, err.Error()))
				return err
			}
			repoPath = resolved
		}
		imageTag := deploymodel.DefaultImageTag(branch)
		gitRefType := normalizeGitRefType(spec.GitRefType)
		buildID, err := e.trigger.TriggerAutomatedBuild(ctx, appctx.AutomatedBuildRequest{
			ProjectID: projectID, Branch: branch, ImageRepository: repoPath, ImageTag: imageTag, GitRefType: gitRefType,
			Variables: VariablesFromRun(run),
		})
		if err != nil {
			jobRun.Status = model.JobRunStatusFailed
			jobRun.ErrorMessage = err.Error()
			failed := failedBuildRun(spec, branch, err.Error())
			failed.ImageTag = imageTag
			failed.ImageRepository = repoPath
			jobRun.Builds = append(jobRun.Builds, failed)
			return err
		}
		jobRun.Builds = append(jobRun.Builds, model.ProjectBuildRun{
			ProjectID: spec.ProjectID, Branch: branch, GitRefType: normalizeGitRefType(spec.GitRefType),
			ImageTag: imageTag, ImageRepository: repoPath,
			BuildID: buildID.String(), Status: model.JobRunStatusRunning,
		})
		if err := e.repo.SaveBuildRef(ctx, &model.WorkflowBuildRef{
			BuildID: buildID, RunID: run.ID, JobName: job.Name, ProjectID: projectID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func parseRegistryID(spec map[string]any) string {
	if spec == nil {
		return ""
	}
	id, _ := spec["registry_id"].(string)
	return strings.TrimSpace(id)
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
		refType, _ := m["git_ref_type"].(string)
		if pid == "" {
			continue
		}
		out = append(out, model.ProjectBuildSpec{ProjectID: pid, Branch: branch, GitRefType: refType})
	}
	return out
}

func ImageRepositoryPath(registryURL, namespace, projectName string) string {
	raw := strings.TrimSpace(registryURL)
	if raw == "" {
		return ""
	}
	parseable := raw
	if !strings.Contains(parseable, "://") {
		parseable = "https://" + parseable
	}
	parsed, err := url.Parse(parseable)
	host := ""
	if err == nil {
		host = parsed.Host
	}
	if host == "" {
		host = strings.Trim(raw, "/")
	}
	ns := strings.Trim(strings.ToLower(strings.TrimSpace(namespace)), "/")
	name := strings.Trim(strings.ToLower(strings.TrimSpace(projectName)), "/")
	if ns == "" || name == "" {
		return strings.ToLower(strings.Trim(host, "/"))
	}
	return strings.ToLower(strings.Trim(host, "/")) + "/" + ns + "/" + name
}

func normalizeGitRefType(gitRefType string) string {
	if strings.TrimSpace(gitRefType) == "tag" {
		return "tag"
	}
	return "branch"
}

func failedBuildRun(spec model.ProjectBuildSpec, branch, message string) model.ProjectBuildRun {
	return model.ProjectBuildRun{
		ProjectID: spec.ProjectID, Branch: branch, GitRefType: normalizeGitRefType(spec.GitRefType),
		Status: model.JobRunStatusFailed, ErrorMessage: message,
	}
}
