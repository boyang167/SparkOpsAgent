package engine

import (
	"encoding/json"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

type GitRunContext struct {
	ProjectID       string
	Event           string
	Ref             string
	RefType         string
	Commit          string
	MergeRequestIID int
}

func PrepareRun(wf *model.ProductWorkflow, runID, source string, input map[string]any, git *GitRunContext) ([]model.WorkflowStage, map[string]any, map[string]any, error) {
	if source == "" {
		source = "manual"
	}
	vars, err := mergeRunVariables(wf, runID, input, git)
	if err != nil {
		return nil, nil, nil, err
	}
	stages := InterpolateStages(cloneStages(wf.Stages), vars)
	if git != nil && git.ProjectID != "" && git.Ref != "" {
		stages = ApplyGitProjectRef(stages, git.ProjectID, git.Ref, git.RefType)
	}
	stages = ApplyRunInput(stages, input)
	if indices, err := stageIndicesFromInput(input); err != nil {
		return nil, nil, nil, err
	} else if len(indices) > 0 {
		stages, err = FilterStagesByIndices(stages, indices)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	stored := map[string]any{}
	for key, value := range input {
		if key == "variables" {
			continue
		}
		stored[key] = value
	}
	stored["variables"] = vars
	trigger := map[string]any{"source": source}
	if git != nil {
		trigger["project_id"] = git.ProjectID
		trigger["event"] = git.Event
		trigger["ref"] = git.Ref
		trigger["commit"] = git.Commit
		if git.MergeRequestIID > 0 {
			trigger["mr_iid"] = git.MergeRequestIID
		}
	}
	return stages, stored, trigger, nil
}

func mergeRunVariables(wf *model.ProductWorkflow, runID string, input map[string]any, git *GitRunContext) (map[string]string, error) {
	merged := map[string]string{}
	if wf == nil {
		wf = &model.ProductWorkflow{}
	}
	for _, item := range wf.Variables {
		merged[item.Key] = item.Value
	}
	provided := stringMap(nil)
	if input != nil {
		provided = stringMap(input["variables"])
		for key, value := range provided {
			merged[key] = value
		}
	}
	missing := make([]string, 0)
	for _, item := range wf.Variables {
		if !item.RuntimeInput {
			continue
		}
		if strings.TrimSpace(merged[item.Key]) == "" {
			missing = append(missing, item.Key)
		}
	}
	if len(missing) > 0 {
		return nil, model.FormatMissingVariables(missing)
	}
	for key, value := range systemVariables(wf, runID, git) {
		merged[key] = value
	}
	return merged, nil
}

func systemVariables(wf *model.ProductWorkflow, runID string, git *GitRunContext) map[string]string {
	name, key, productID := "", "", ""
	if wf != nil {
		name = wf.Name
		key = wf.Key
		productID = wf.ProductID.String()
	}
	vars := map[string]string{
		"WORKFLOW_NAME": name,
		"WORKFLOW_KEY":  key,
		"PRODUCT_ID":    productID,
		"RUN_ID":        runID,
		"GIT_BRANCH":    "",
		"GIT_TAG":       "",
		"GIT_COMMIT":    "",
		"GIT_EVENT":     "",
	}
	if git == nil {
		return vars
	}
	vars["GIT_EVENT"] = git.Event
	vars["GIT_COMMIT"] = git.Commit
	if git.RefType == "tag" {
		vars["GIT_TAG"] = git.Ref
		return vars
	}
	vars["GIT_BRANCH"] = git.Ref
	return vars
}

func ApplyGitProjectRef(stages []model.WorkflowStage, projectID, ref, refType string) []model.WorkflowStage {
	projectID = strings.TrimSpace(projectID)
	ref = strings.TrimSpace(ref)
	if projectID == "" || ref == "" {
		return stages
	}
	switch strings.TrimSpace(refType) {
	case "tag", "pr":
	default:
		refType = "branch"
	}
	for si := range stages {
		for ji := range stages[si].Jobs {
			job := &stages[si].Jobs[ji]
			if job.Type != model.JobTypeSparkopsBuild || job.Spec == nil {
				continue
			}
			raw, ok := job.Spec["project_builds"].([]any)
			if !ok {
				continue
			}
			for i, item := range raw {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id, _ := row["project_id"].(string)
				if id != projectID {
					continue
				}
				row["branch"] = ref
				row["git_ref_type"] = refType
				raw[i] = row
			}
			job.Spec["project_builds"] = raw
		}
	}
	return stages
}

func stageIndicesFromInput(input map[string]any) ([]int, error) {
	if input == nil {
		return nil, nil
	}
	raw, ok := input["stage_indices"]
	if !ok || raw == nil {
		return nil, nil
	}
	switch typed := raw.(type) {
	case []int:
		return typed, nil
	case []any:
		out := make([]int, 0, len(typed))
		for _, item := range typed {
			switch n := item.(type) {
			case float64:
				out = append(out, int(n))
			case int:
				out = append(out, n)
			case int64:
				out = append(out, int(n))
			default:
				return nil, domain.ErrInvalidWorkflowInput
			}
		}
		return out, nil
	default:
		return nil, domain.ErrInvalidWorkflowInput
	}
}

// FilterStagesByIndices keeps only the selected stages in original workflow order.
// StageIndicesFromRunInput reads optional partial-run stage indices persisted on a run.
func StageIndicesFromRunInput(raw json.RawMessage) ([]int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, nil
	}
	return stageIndicesFromInput(input)
}

// WorkflowStagesForRun resolves the stage definitions that belong to this run,
// re-applying the variables and runtime overrides persisted on the run so that
// stages advanced after a reload see the same specs as the first stage.
func WorkflowStagesForRun(wf *model.ProductWorkflow, run *model.ProductWorkflowRun) []model.WorkflowStage {
	stages := selectRunStages(wf, run)
	if run == nil || len(run.Input) == 0 || len(stages) == 0 {
		return stages
	}
	var input map[string]any
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return stages
	}
	stages = InterpolateStages(cloneStages(stages), VariablesFromRun(run))
	return ApplyRunInput(stages, input)
}

func selectRunStages(wf *model.ProductWorkflow, run *model.ProductWorkflowRun) []model.WorkflowStage {
	if wf == nil {
		return nil
	}
	var indices []int
	var err error
	if run != nil {
		indices, err = StageIndicesFromRunInput(run.Input)
	}
	if err != nil || len(indices) == 0 {
		if run != nil && len(run.StageRuns) > 0 && len(run.StageRuns) < len(wf.Stages) {
			return append([]model.WorkflowStage(nil), wf.Stages[:len(run.StageRuns)]...)
		}
		return wf.Stages
	}
	out := make([]model.WorkflowStage, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(wf.Stages) {
			out = append(out, wf.Stages[idx])
		}
	}
	if len(out) == 0 {
		return wf.Stages
	}
	return out
}

func FilterStagesByIndices(stages []model.WorkflowStage, indices []int) ([]model.WorkflowStage, error) {
	if len(indices) == 0 {
		return stages, nil
	}
	selected := make(map[int]struct{}, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(stages) {
			return nil, domain.ErrInvalidWorkflowInput
		}
		selected[idx] = struct{}{}
	}
	out := make([]model.WorkflowStage, 0, len(indices))
	for i, stage := range stages {
		if _, ok := selected[i]; ok {
			out = append(out, stage)
		}
	}
	if len(out) == 0 {
		return nil, domain.ErrInvalidWorkflowInput
	}
	return out, nil
}

func stringMap(raw any) map[string]string {
	out := map[string]string{}
	switch typed := raw.(type) {
	case map[string]string:
		for key, value := range typed {
			out[key] = strings.TrimSpace(value)
		}
	case map[string]any:
		for key, value := range typed {
			text, ok := value.(string)
			if !ok {
				continue
			}
			out[key] = strings.TrimSpace(text)
		}
	}
	return out
}
