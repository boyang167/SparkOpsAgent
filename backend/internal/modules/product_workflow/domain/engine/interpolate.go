package engine

import (
	"encoding/json"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

func Interpolate(value string, vars map[string]string) string {
	return interpolate(value, vars, true)
}

// interpolate replaces {{.key}} always and {{key}} only when bare is true.
func interpolate(value string, vars map[string]string, bare bool) string {
	if value == "" || !strings.Contains(value, "{{") {
		return value
	}
	return placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := placeholderPattern.FindStringSubmatch(match)
		if len(parts) != 3 || (parts[1] == "" && !bare) {
			return match
		}
		if replacement, ok := vars[parts[2]]; ok {
			return replacement
		}
		return match
	})
}

func InterpolateStages(stages []model.WorkflowStage, vars map[string]string) []model.WorkflowStage {
	if len(stages) == 0 || len(vars) == 0 {
		return stages
	}
	raw, err := json.Marshal(stages)
	if err != nil {
		return stages
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return stages
	}
	walked := generic
	for i := 0; i < 3; i++ {
		walked = interpolateValue(walked, vars, true)
	}
	outRaw, err := json.Marshal(walked)
	if err != nil {
		return stages
	}
	var out []model.WorkflowStage
	if err := json.Unmarshal(outRaw, &out); err != nil {
		return stages
	}
	return out
}

func interpolateValue(value any, vars map[string]string, bare bool) any {
	switch typed := value.(type) {
	case string:
		return interpolate(typed, vars, bare)
	case []any:
		for i, item := range typed {
			typed[i] = interpolateValue(item, vars, bare)
		}
		return typed
	case map[string]any:
		for key, item := range typed {
			typed[key] = interpolateValue(item, vars, bare && !isRawContentKey(key))
		}
		return typed
	default:
		return value
	}
}

func VariablesFromRun(run *model.ProductWorkflowRun) map[string]string {
	if run == nil || len(run.Input) == 0 {
		return nil
	}
	var doc struct {
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(run.Input, &doc); err != nil {
		return nil
	}
	out := map[string]string{}
	for key, value := range doc.Variables {
		text, ok := value.(string)
		if !ok {
			continue
		}
		out[key] = text
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
