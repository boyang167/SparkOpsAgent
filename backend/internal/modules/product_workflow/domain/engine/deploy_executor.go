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
	trigger  appctx.DeploymentTrigger
	creator  appctx.VersionCreator
	reader   appctx.ProjectVersionReader
	repo     repository.ProductWorkflowRepository
}

func NewDeployExecutor(
	trigger appctx.DeploymentTrigger,
	creator appctx.VersionCreator,
	reader appctx.ProjectVersionReader,
	repo repository.ProductWorkflowRepository,
) *DeployExecutor {
	return &DeployExecutor{trigger: trigger, creator: creator, reader: reader, repo: repo}
}

func (e *DeployExecutor) RunJob(
	ctx context.Context,
	wf *model.ProductWorkflow,
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
	if deployArtifactSource(job.Type, spec) == "version" {
		return e.runVersionDeploy(ctx, wf, run, stageIndex, jobIndex, job, envID, refJobName, spec)
	}
	return e.runBuildDeploy(ctx, run, stageIndex, jobIndex, job, envID, refJobName, spec)
}

func deployArtifactSource(jobType string, spec map[string]any) string {
	if jobType == model.JobTypeSparkopsDeployVersion {
		return "version"
	}
	raw, _ := spec["artifact_source"].(string)
	if strings.TrimSpace(raw) == "version" {
		return "version"
	}
	return "build"
}

func (e *DeployExecutor) runBuildDeploy(
	ctx context.Context,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
	envID kernel.EnvironmentID,
	refJobName string,
	spec map[string]any,
) error {
	jobRun := &run.StageRuns[stageIndex].Jobs[jobIndex]
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
		gitRefType, imageTag, imageRepository := deployImage(target, branch)
		deployID, triggerErr := e.trigger.TriggerForEnvironment(ctx, appctx.WorkflowDeploymentRequest{
			ProjectID: projectID, EnvironmentID: envID,
			GitRefType: gitRefType, GitRefValue: branch, ImageTag: imageTag, ImageRepository: imageRepository,
			DeployContents: parseDeployContents(spec), ConfigResources: configResourcesFor(spec, target.ProjectID),
		})
		if triggerErr != nil {
			jobRun.Status = model.JobRunStatusFailed
			jobRun.ErrorMessage = triggerErr.Error()
			jobRun.Deploys = append(jobRun.Deploys, model.ProjectDeployRun{
				ProjectID: target.ProjectID, EnvironmentID: envID.String(),
				ImageTag: imageTag, ImageRepository: imageRepository,
				Status: model.JobRunStatusFailed, ErrorMessage: triggerErr.Error(),
			})
			return triggerErr
		}
		jobRun.Deploys = append(jobRun.Deploys, model.ProjectDeployRun{
			ProjectID: target.ProjectID, EnvironmentID: envID.String(),
			DeploymentRunID: deployID.String(), ImageTag: imageTag, ImageRepository: imageRepository,
			Status: model.JobRunStatusRunning,
		})
		if err := e.repo.SaveDeployRef(ctx, &model.WorkflowDeployRef{
			DeploymentRunID: deployID, RunID: run.ID, JobName: job.Name, ProjectID: projectID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func versionSourceKind(jobType string, spec map[string]any, run *model.ProductWorkflowRun, refJobName string) string {
	if kind, _ := spec["source_kind"].(string); kind == "build" || kind == "create-version" {
		return kind
	}
	switch resolveUpstreamJobType(run, refJobName) {
	case model.JobTypeSparkopsBuild:
		return "build"
	case model.JobTypeSparkopsCreateVersion:
		return "create-version"
	}
	if len(resolveFromJobVersions(run, refJobName)) > 0 {
		return "create-version"
	}
	if jobType == model.JobTypeSparkopsDeployVersion {
		return "create-version"
	}
	return "build"
}

func (e *DeployExecutor) runVersionDeploy(
	ctx context.Context,
	wf *model.ProductWorkflow,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
	envID kernel.EnvironmentID,
	refJobName string,
	spec map[string]any,
) error {
	switch versionSourceKind(job.Type, spec, run, refJobName) {
	case "create-version":
		return e.runVersionDeployFromCreateJob(ctx, run, stageIndex, jobIndex, job, envID, refJobName)
	default:
		return e.runVersionDeployFromBuild(ctx, wf, run, stageIndex, jobIndex, job, envID, refJobName, spec)
	}
}

func (e *DeployExecutor) runVersionDeployFromCreateJob(
	ctx context.Context,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
	envID kernel.EnvironmentID,
	refJobName string,
) error {
	jobRun := &run.StageRuns[stageIndex].Jobs[jobIndex]
	targets := resolveFromJobVersions(run, refJobName)
	if len(targets) == 0 {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "no successful versions from job " + refJobName
		return fmt.Errorf("no version outputs")
	}
	return e.deployVersionTargets(ctx, run, stageIndex, jobIndex, job, envID, targets)
}

func (e *DeployExecutor) runVersionDeployFromBuild(
	ctx context.Context,
	wf *model.ProductWorkflow,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
	envID kernel.EnvironmentID,
	refJobName string,
	spec map[string]any,
) error {
	if e.creator == nil {
		return fmt.Errorf("version creator unavailable")
	}
	if wf == nil {
		return fmt.Errorf("workflow context required for version deploy")
	}
	jobRun := &run.StageRuns[stageIndex].Jobs[jobIndex]
	versionTag, _ := spec["version_tag"].(string)
	versionTag = strings.TrimSpace(versionTag)
	if versionTag == "" {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "version_tag is required for delivery version deploy"
		return fmt.Errorf("version_tag is required")
	}
	builds := resolveFromJobBuilds(run, refJobName)
	if len(builds) == 0 {
		jobRun.Status = model.JobRunStatusFailed
		jobRun.ErrorMessage = "no successful builds from job " + refJobName
		return fmt.Errorf("no build outputs")
	}
	targets := make([]model.ProjectVersionRun, 0, len(builds))
	for _, target := range builds {
		projectID, err := kernel.ParseProjectID(target.ProjectID)
		if err != nil {
			return err
		}
		branch := target.Branch
		if branch == "" {
			branch = "main"
		}
		gitRefType, imageTag, imageRepository := deployImage(target, branch)
		versionID, err := e.creator.CreateFromWorkflow(ctx, appctx.CreateVersionRequest{
			WorkspaceID: wf.WorkspaceID, ProjectID: projectID, ProductID: run.ProductID,
			VersionTag: versionTag, WorkflowRunID: run.ID, WorkflowJobName: job.Name,
			GitRefType: gitRefType, GitRefValue: branch,
			ImageTag: imageTag, ImageRepository: imageRepository, BuildID: target.BuildID,
			CreatedBy: wf.CreatedBy,
		})
		if err != nil {
			jobRun.Status = model.JobRunStatusFailed
			jobRun.ErrorMessage = err.Error()
			return err
		}
		targets = append(targets, model.ProjectVersionRun{
			ProjectID: target.ProjectID, VersionID: versionID.String(),
			VersionTag: versionTag, Status: model.JobRunStatusSuccess,
		})
	}
	jobRun.Versions = targets
	return e.deployVersionTargets(ctx, run, stageIndex, jobIndex, job, envID, targets)
}

func (e *DeployExecutor) deployVersionTargets(
	ctx context.Context,
	run *model.ProductWorkflowRun,
	stageIndex int,
	jobIndex int,
	job model.WorkflowJob,
	envID kernel.EnvironmentID,
	targets []model.ProjectVersionRun,
) error {
	jobRun := &run.StageRuns[stageIndex].Jobs[jobIndex]
	jobRun.Deploys = make([]model.ProjectDeployRun, 0, len(targets))
	jobRun.Status = model.JobRunStatusRunning
	run.StageRuns[stageIndex].Status = model.JobRunStatusRunning

	for _, target := range targets {
		projectID, parseErr := kernel.ParseProjectID(target.ProjectID)
		if parseErr != nil {
			return parseErr
		}
		versionID, parseErr := kernel.ParseProjectVersionID(target.VersionID)
		if parseErr != nil {
			return parseErr
		}
		gitRefType := "branch"
		gitRefValue := "main"
		imageTag := ""
		imageRepository := ""
		if e.reader != nil {
			snapshot, snapErr := e.reader.LoadSnapshot(ctx, versionID)
			if snapErr == nil {
				gitRefType = snapshot.GitRefType
				gitRefValue = snapshot.GitRefValue
				if len(snapshot.Images) > 0 {
					imageTag = snapshot.Images[0].Tag
					imageRepository = snapshot.Images[0].Image
				}
			}
		}
		deployID, triggerErr := e.trigger.TriggerForEnvironment(ctx, appctx.WorkflowDeploymentRequest{
			ProjectID: projectID, EnvironmentID: envID,
			GitRefType: gitRefType, GitRefValue: gitRefValue,
			ImageTag: imageTag, ImageRepository: imageRepository,
			ProjectVersionID: &versionID,
		})
		if triggerErr != nil {
			jobRun.Status = model.JobRunStatusFailed
			jobRun.ErrorMessage = triggerErr.Error()
			jobRun.Deploys = append(jobRun.Deploys, model.ProjectDeployRun{
				ProjectID: target.ProjectID, EnvironmentID: envID.String(),
				VersionID: target.VersionID, ImageTag: imageTag, ImageRepository: imageRepository,
				Status: model.JobRunStatusFailed, ErrorMessage: triggerErr.Error(),
			})
			return triggerErr
		}
		jobRun.Deploys = append(jobRun.Deploys, model.ProjectDeployRun{
			ProjectID: target.ProjectID, EnvironmentID: envID.String(),
			DeploymentRunID: deployID.String(), VersionID: target.VersionID,
			ImageTag: imageTag, ImageRepository: imageRepository,
			Status: model.JobRunStatusRunning,
		})
		if err := e.repo.SaveDeployRef(ctx, &model.WorkflowDeployRef{
			DeploymentRunID: deployID, RunID: run.ID, JobName: job.Name, ProjectID: projectID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func resolveUpstreamJobType(run *model.ProductWorkflowRun, jobName string) string {
	for _, stage := range run.StageRuns {
		for _, job := range stage.Jobs {
			if job.Name == jobName {
				return job.Type
			}
		}
	}
	return ""
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

func deployImage(target model.ProjectBuildRun, branch string) (gitRefType, imageTag, imageRepository string) {
	imageTag = strings.TrimSpace(target.ImageTag)
	if imageTag == "" {
		imageTag = deploymodel.DefaultImageTag(branch)
	}
	switch strings.TrimSpace(target.GitRefType) {
	case "tag":
		gitRefType = "tag"
	case "pr":
		gitRefType = "pr"
	default:
		gitRefType = "branch"
	}
	return gitRefType, imageTag, strings.TrimSpace(target.ImageRepository)
}

func parseDeployContents(spec map[string]any) []string {
	raw, ok := spec["deploy_contents"]
	if !ok {
		return []string{"image"}
	}
	items, ok := raw.([]any)
	if !ok {
		return []string{"image"}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		value, _ := item.(string)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return []string{"image"}
	}
	return out
}

func wantsConfigSync(spec map[string]any) bool {
	for _, item := range parseDeployContents(spec) {
		if item == "config" {
			return true
		}
	}
	return false
}

func configResourcesFor(spec map[string]any, projectID string) []appctx.WorkflowConfigResource {
	if !wantsConfigSync(spec) {
		return nil
	}
	raw, ok := spec["config_resources"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]appctx.WorkflowConfigResource, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pid, _ := row["project_id"].(string)
		if strings.TrimSpace(pid) != "" && pid != projectID {
			continue
		}
		kind, _ := row["kind"].(string)
		name, _ := row["name"].(string)
		yamlDoc, _ := row["yaml"].(string)
		if kind != "ConfigMap" && kind != "Secret" {
			continue
		}
		if strings.TrimSpace(name) == "" {
			continue
		}
		out = append(out, appctx.WorkflowConfigResource{Kind: kind, Name: strings.TrimSpace(name), YAML: yamlDoc})
	}
	return out
}
