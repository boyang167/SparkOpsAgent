package engine

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

var interpolatePattern = regexp.MustCompile(`\{\{\.([A-Za-z][A-Za-z0-9_]*)\}\}`)

func Interpolate(value string, vars map[string]string) string {
	if value == "" || !strings.Contains(value, "{{.") {
		return value
	}
	return interpolatePattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := interpolatePattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		if replacement, ok := vars[parts[1]]; ok {
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
		walked = interpolateValue(walked, vars)
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

func interpolateValue(value any, vars map[string]string) any {
	switch typed := value.(type) {
	case string:
		return Interpolate(typed, vars)
	case []any:
		for i, item := range typed {
			typed[i] = interpolateValue(item, vars)
		}
		return typed
	case map[string]any:
		for key, item := range typed {
			if key == "yaml" || key == "config_resources" {
				continue
			}
			typed[key] = interpolateValue(item, vars)
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
