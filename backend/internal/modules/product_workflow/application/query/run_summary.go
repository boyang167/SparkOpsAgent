package query

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

func runSummaryView(run *model.ProductWorkflowRun, runNumber int) map[string]any {
	out := map[string]any{
		"id": run.ID.String(), "status": run.Status, "current_stage": run.CurrentStage,
		"created_at": run.CreatedAt.Format(time.RFC3339),
	}
	if runNumber > 0 {
		out["run_number"] = runNumber
	}
	if run.StartedAt != nil {
		out["started_at"] = run.StartedAt.Format(time.RFC3339)
	}
	if run.CompletedAt != nil {
		out["completed_at"] = run.CompletedAt.Format(time.RFC3339)
	}
	if ms := runDurationMs(run); ms > 0 {
		out["duration_ms"] = ms
	}
	if remark := runRemark(run.Input); remark != "" {
		out["remark"] = remark
	}
	if source, actor := runTriggerMeta(run.Trigger); source != "" {
		out["trigger_source"] = source
		if actor != "" {
			out["executor_id"] = actor
		}
	}
	if branches := runBranches(run.Input); len(branches) > 0 {
		out["branches"] = branches
	}
	if builds := runBuildServices(run.StageRuns); len(builds) > 0 {
		out["build_services"] = builds
	}
	if deploys := runDeployServices(run.StageRuns); len(deploys) > 0 {
		out["deploy_services"] = deploys
	}
	return out
}

func runDurationMs(run *model.ProductWorkflowRun) int64 {
	if run == nil {
		return 0
	}
	if run.StartedAt != nil && run.CompletedAt != nil {
		return run.CompletedAt.Sub(*run.StartedAt).Milliseconds()
	}
	if run.StartedAt != nil && (run.Status == model.RunStatusRunning || run.Status == model.RunStatusPaused) {
		return time.Since(*run.StartedAt).Milliseconds()
	}
	return 0
}

func runTriggerMeta(raw json.RawMessage) (source, actor string) {
	if len(raw) == 0 {
		return "", ""
	}
	var trigger map[string]any
	if err := json.Unmarshal(raw, &trigger); err != nil {
		return "", ""
	}
	source, _ = trigger["source"].(string)
	source = strings.TrimSpace(source)
	actor, _ = trigger["started_by"].(string)
	actor = strings.TrimSpace(actor)
	return source, actor
}

func runBranches(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil
	}
	refTypes, _ := stored["ref_type_overrides"].(map[string]any)
	overrides, _ := stored["branch_overrides"].(map[string]any)
	seen := map[string]struct{}{}
	out := make([]map[string]any, 0)
	appendBranch := func(projectID, branch, refType string) {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			return
		}
		key := projectID + "|" + branch + "|" + refType
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		row := map[string]any{"branch": branch}
		if projectID != "" {
			row["project_id"] = projectID
		}
		if refType = strings.TrimSpace(refType); refType != "" {
			row["ref_type"] = refType
		}
		out = append(out, row)
	}
	for key, value := range overrides {
		branch, _ := value.(string)
		projectID := ""
		if parts := strings.SplitN(key, ":", 2); len(parts) == 2 {
			projectID = parts[1]
		}
		refType, _ := refTypes[key].(string)
		appendBranch(projectID, branch, refType)
	}
	if len(out) == 0 {
		if branch, _ := stored["branch"].(string); strings.TrimSpace(branch) != "" {
			refType, _ := stored["ref_type"].(string)
			appendBranch("", branch, refType)
		}
	}
	return out
}

func runBuildServices(stages []model.StageRun) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, stage := range stages {
		for _, job := range stage.Jobs {
			if job.Type != model.JobTypeSparkopsBuild {
				continue
			}
			for _, build := range job.Builds {
				id := strings.TrimSpace(build.ProjectID)
				if id == "" {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				out = append(out, id)
			}
		}
	}
	return out
}

func runDeployServices(stages []model.StageRun) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, stage := range stages {
		for _, job := range stage.Jobs {
			if job.Type != model.JobTypeSparkopsDeploy {
				continue
			}
			for _, deploy := range job.Deploys {
				id := strings.TrimSpace(deploy.ProjectID)
				if id == "" {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				out = append(out, id)
			}
		}
	}
	return out
}
