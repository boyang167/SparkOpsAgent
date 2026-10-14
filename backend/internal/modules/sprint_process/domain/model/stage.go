package model

import "strings"

const (
	TriggerModeManual = "manual"
	TriggerModeAuto   = "auto_on_enter"
)

type StageWorkflowBinding struct {
	WorkflowID        string            `json:"workflow_id"`
	WorkflowName      string            `json:"workflow_name,omitempty"`
	TriggerMode       string            `json:"trigger_mode"`
	DefaultParameters map[string]string `json:"default_parameters,omitempty"`
	RequiredForExit   bool              `json:"required_for_exit,omitempty"`
}

type StageGatePolicy struct {
	RequireBranch          bool     `json:"require_branch,omitempty"`
	RequireWorkflowSuccess []string `json:"require_workflow_success,omitempty"`
	RequireJiraStatus      []string `json:"require_jira_status,omitempty"`
	RequireApproval        bool     `json:"require_approval,omitempty"`
	ProtectedBranches      []string `json:"protected_branches,omitempty"`
}

type SprintStage struct {
	ID          string                 `json:"id"`
	Key         string                 `json:"key"`
	Name        string                 `json:"name"`
	SortOrder   int                    `json:"sort_order"`
	Color       string                 `json:"color,omitempty"`
	WipLimit    int                    `json:"wip_limit,omitempty"`
	EntryPolicy *StageGatePolicy       `json:"entry_policy,omitempty"`
	Workflows   []StageWorkflowBinding `json:"workflows,omitempty"`
}

// NormalizeStageWorkflowBindings drops empty bindings and coerces unknown trigger modes to manual.
func NormalizeStageWorkflowBindings(in []StageWorkflowBinding) []StageWorkflowBinding {
	out := make([]StageWorkflowBinding, 0, len(in))
	for _, b := range in {
		b.WorkflowID = strings.TrimSpace(b.WorkflowID)
		if b.WorkflowID == "" {
			continue
		}
		if b.TriggerMode != TriggerModeAuto {
			b.TriggerMode = TriggerModeManual
		}
		out = append(out, b)
	}
	return out
}
