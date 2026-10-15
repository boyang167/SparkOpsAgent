package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

// placeholderPattern matches {{.KEY}} (run variables) and {{key}} (template leftovers).
var placeholderPattern = regexp.MustCompile(`\{\{(\.?)([A-Za-z][A-Za-z0-9_]*)\}\}`)

// rawContentKeys hold verbatim file bodies that may carry their own {{name}} text;
// only {{.KEY}} is recognised inside them.
var rawContentKeys = map[string]struct{}{
	"content":  {},
	"yaml":     {},
	"manifest": {},
	"template": {},
}

func isRawContentKey(key string) bool {
	_, ok := rawContentKeys[key]
	return ok
}

type RuntimeInputRef struct {
	StageIndex int    `json:"stage_index"`
	StageName  string `json:"stage_name"`
	JobName    string `json:"job_name"`
	JobType    string `json:"job_type"`
	Field      string `json:"field"`
}

type RuntimeInput struct {
	Key      string            `json:"key"`
	Label    string            `json:"label"`
	Declared bool              `json:"declared"`
	Secret   bool              `json:"secret,omitempty"`
	Refs     []RuntimeInputRef `json:"refs,omitempty"`
}

// RuntimeInputs lists the values a run must provide once input overrides are applied.
func RuntimeInputs(wf *model.ProductWorkflow, input map[string]any) []RuntimeInput {
	out := []RuntimeInput{}
	if wf == nil {
		return out
	}
	declared := map[string]model.WorkflowVariable{}
	for _, item := range wf.Variables {
		if key := strings.TrimSpace(item.Key); key != "" {
			declared[key] = item
		}
	}
	system := systemVariables(nil, "", nil)
	index := map[string]int{}
	seenRef := map[string]struct{}{}
	add := func(key string, ref *RuntimeInputRef) {
		pos, ok := index[key]
		if !ok {
			item := RuntimeInput{Key: key, Label: key}
			if variable, isDeclared := declared[key]; isDeclared {
				item.Declared = true
				item.Secret = variable.Secret
				if desc := strings.TrimSpace(variable.Description); desc != "" {
					item.Label = desc
				}
			}
			out = append(out, item)
			pos = len(out) - 1
			index[key] = pos
		}
		if ref == nil {
			return
		}
		refKey := fmt.Sprintf("%s|%d|%s|%s", key, ref.StageIndex, ref.JobName, ref.Field)
		if _, dup := seenRef[refKey]; dup {
			return
		}
		seenRef[refKey] = struct{}{}
		out[pos].Refs = append(out[pos].Refs, *ref)
	}
	for _, item := range wf.Variables {
		if key := strings.TrimSpace(item.Key); key != "" && item.RuntimeInput {
			add(key, nil)
		}
	}
	stages := ApplyRunInput(cloneStages(wf.Stages), input)
	scanStagePlaceholders(stages, func(key string, ref RuntimeInputRef) {
		if _, ok := system[key]; ok {
			return
		}
		if variable, ok := declared[key]; ok && !variable.RuntimeInput {
			return
		}
		add(key, &ref)
	})
	return out
}

// UnresolvedPlaceholders lists placeholder keys still present in stage specs, in first-seen order.
func UnresolvedPlaceholders(stages []model.WorkflowStage) []string {
	keys := []string{}
	seen := map[string]struct{}{}
	scanStagePlaceholders(stages, func(key string, _ RuntimeInputRef) {
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	})
	return keys
}

func scanStagePlaceholders(stages []model.WorkflowStage, visit func(key string, ref RuntimeInputRef)) {
	for si, stage := range stages {
		for _, job := range stage.Jobs {
			spec := genericSpec(job.Spec)
			for _, field := range sortedKeys(spec) {
				ref := RuntimeInputRef{
					StageIndex: si, StageName: stage.Name, JobName: job.Name, JobType: job.Type, Field: field,
				}
				scanPlaceholderValue(spec[field], !isRawContentKey(field), func(key string) { visit(key, ref) })
			}
		}
	}
}

func scanPlaceholderValue(value any, bare bool, visit func(key string)) {
	switch typed := value.(type) {
	case string:
		for _, match := range placeholderPattern.FindAllStringSubmatch(typed, -1) {
			if match[1] == "" && !bare {
				continue
			}
			visit(match[2])
		}
	case []any:
		for _, item := range typed {
			scanPlaceholderValue(item, bare, visit)
		}
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			scanPlaceholderValue(typed[key], bare && !isRawContentKey(key), visit)
		}
	}
}

// genericSpec normalises typed slices (e.g. []map[string]any) into plain JSON values.
func genericSpec(spec map[string]any) map[string]any {
	raw, err := json.Marshal(spec)
	if err != nil {
		return spec
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return spec
	}
	return out
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
