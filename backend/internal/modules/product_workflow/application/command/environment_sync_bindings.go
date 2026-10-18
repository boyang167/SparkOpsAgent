package command

import (
	"context"
	"strings"

	deploymodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func environmentSyncJobs(stages []model.WorkflowStage) []model.WorkflowJob {
	out := make([]model.WorkflowJob, 0)
	for _, stage := range stages {
		for _, job := range stage.Jobs {
			if job.Type == model.JobTypeSparkopsEnvironmentSync {
				out = append(out, job)
			}
		}
	}
	return out
}

func previewIDForJob(stored map[string]any, jobName string) (string, bool) {
	if stored == nil {
		return "", false
	}
	previewIDs, _ := stored["environment_sync_preview_ids"].(map[string]any)
	rawID, _ := previewIDs[jobName].(string)
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return "", false
	}
	return rawID, true
}

func validateEnvironmentSyncSnapshotAlignment(snapshot *appctx.EnvironmentSyncSnapshot, job model.WorkflowJob) error {
	if snapshot == nil {
		return domain.ErrInvalidWorkflowInput
	}
	sourceRaw, _ := job.Spec["source_environment_id"].(string)
	targetRaw, _ := job.Spec["target_environment_id"].(string)
	sourceID, err := kernel.ParseEnvironmentID(strings.TrimSpace(sourceRaw))
	if err != nil {
		return domain.ErrInvalidWorkflowInput
	}
	targetID, err := kernel.ParseEnvironmentID(strings.TrimSpace(targetRaw))
	if err != nil {
		return domain.ErrInvalidWorkflowInput
	}
	if snapshot.SourceEnvironmentID != sourceID || snapshot.TargetEnvironmentID != targetID {
		return domain.ErrInvalidWorkflowInput
	}
	expected := environmentSyncProjectsFromSpec(job.Spec)
	if len(expected) != len(snapshot.Projects) {
		return domain.ErrInvalidWorkflowInput
	}
	seen := map[kernel.ProjectID]appctx.EnvironmentSyncProjectOption{}
	for _, project := range expected {
		seen[project.ProjectID] = project
	}
	for _, project := range snapshot.Projects {
		want, ok := seen[project.ProjectID]
		if !ok {
			return domain.ErrInvalidWorkflowInput
		}
		if want.SyncImage != project.Option.SyncImage ||
			want.SyncYAML != project.Option.SyncYAML ||
			want.FirstSync != project.Option.FirstSync ||
			!sameSyncResourceKinds(want.SyncResourceKinds, project.Option.SyncResourceKinds) ||
			!sameResourceOverrides(want.ResourceOverrides, project.Option.ResourceOverrides) {
			return domain.ErrInvalidWorkflowInput
		}
	}
	return nil
}

func environmentSyncProjectsFromSpec(spec map[string]any) []appctx.EnvironmentSyncProjectOption {
	if spec == nil {
		return nil
	}
	rawProjects, _ := spec["projects"].([]any)
	out := make([]appctx.EnvironmentSyncProjectOption, 0, len(rawProjects))
	for _, item := range rawProjects {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		projectRaw, _ := row["project_id"].(string)
		projectID, err := kernel.ParseProjectID(strings.TrimSpace(projectRaw))
		if err != nil {
			continue
		}
		syncImage := row["sync_image"] != false
		syncYAML, _ := row["sync_yaml"].(bool)
		firstSync, _ := row["first_sync"].(bool)
		kinds := parseSyncResourceKinds(row["sync_resource_kinds"])
		overrides := parseResourceOverrides(row["resource_overrides"])
		option := appctx.EnvironmentSyncProjectOption{
			ProjectID:         projectID,
			SyncImage:         syncImage,
			SyncYAML:          syncYAML,
			SyncResourceKinds: kinds,
			ResourceOverrides: overrides,
			FirstSync:         firstSync,
		}
		normalizeAppctxEnvironmentSyncOption(&option)
		out = append(out, option)
	}
	return out
}

func collectEnvironmentSyncBindings(
	ctx context.Context,
	snapshots appctx.EnvironmentSyncSnapshots,
	wf *model.ProductWorkflow,
	stages []model.WorkflowStage,
	stored map[string]any,
	runID kernel.ProductWorkflowRunID,
) ([]appctx.EnvironmentSyncSnapshotBinding, error) {
	jobs := environmentSyncJobs(stages)
	if len(jobs) == 0 {
		return nil, nil
	}
	if snapshots == nil {
		return nil, domain.ErrInvalidWorkflowInput
	}
	bindings := make([]appctx.EnvironmentSyncSnapshotBinding, 0, len(jobs))
	for _, job := range jobs {
		rawID, ok := previewIDForJob(stored, job.Name)
		if !ok {
			return nil, domain.ErrInvalidWorkflowInput
		}
		snapshotID, err := kernel.ParseEnvironmentSyncSnapshotID(rawID)
		if err != nil {
			return nil, domain.ErrInvalidWorkflowInput
		}
		if err := snapshots.Validate(ctx, snapshotID, wf.ProductID, wf.ID, job.Name); err != nil {
			return nil, domain.ErrInvalidWorkflowInput
		}
		snapshot, err := snapshots.LoadPreview(ctx, snapshotID)
		if err != nil {
			return nil, domain.ErrInvalidWorkflowInput
		}
		if err := validateEnvironmentSyncSnapshotAlignment(snapshot, job); err != nil {
			return nil, err
		}
		bindings = append(bindings, appctx.EnvironmentSyncSnapshotBinding{
			SnapshotID: snapshotID,
			RunID:      runID,
		})
	}
	return bindings, nil
}

func parseSyncResourceKinds(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		kind, ok := item.(string)
		if !ok || strings.TrimSpace(kind) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(kind))
	}
	return out
}

func normalizeAppctxEnvironmentSyncOption(option *appctx.EnvironmentSyncProjectOption) {
	if option == nil {
		return
	}
	modelOption := deploymodel.EnvironmentSyncProjectOption{
		ProjectID:         option.ProjectID,
		SyncImage:         option.SyncImage,
		SyncYAML:          option.SyncYAML,
		SyncResourceKinds: append([]string(nil), option.SyncResourceKinds...),
		FirstSync:         option.FirstSync,
	}
	deploymodel.NormalizeEnvironmentSyncOption(&modelOption)
	option.SyncResourceKinds = modelOption.SyncResourceKinds
}

func sameSyncResourceKinds(a, b []string) bool {
	return deploymodel.SameSyncResourceKinds(a, b)
}

func parseResourceOverrides(raw any) map[string]string {
	row, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(row))
	for key, value := range row {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		out[key] = text
	}
	return out
}

func sameResourceOverrides(a, b map[string]string) bool {
	return deploymodel.SameResourceOverrides(a, b)
}
