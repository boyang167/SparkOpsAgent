package engine

import (
	"math"
	"strings"
)

type ApprovalSpec struct {
	Provider          string
	ApproverIDs       []string
	RequiredApprovals int
	TimeoutMinutes    int
}

func ParseApprovalSpec(spec map[string]any) ApprovalSpec {
	if spec == nil {
		return ApprovalSpec{RequiredApprovals: 1, TimeoutMinutes: 60}
	}
	approverIDs := make([]string, 0)
	switch raw := spec["approver_ids"].(type) {
	case []any:
		for _, item := range raw {
			if value, ok := item.(string); ok {
				value = strings.TrimSpace(value)
				if value != "" {
					approverIDs = append(approverIDs, value)
				}
			}
		}
	case []string:
		for _, value := range raw {
			value = strings.TrimSpace(value)
			if value != "" {
				approverIDs = append(approverIDs, value)
			}
		}
	}
	required := intFromAny(spec["required_approvals"], 1)
	if required <= 0 {
		required = 1
	}
	if len(approverIDs) > 0 {
		required = int(math.Min(float64(required), float64(len(approverIDs))))
	}
	timeout := intFromAny(spec["timeout_minutes"], 0)
	if timeout <= 0 {
		if strategy, ok := spec["strategy"].(map[string]any); ok {
			timeout = intFromAny(strategy["timeout_minutes"], 0)
		}
	}
	if timeout <= 0 {
		timeout = 60
	}
	provider, _ := spec["provider"].(string)
	if strings.TrimSpace(provider) == "" {
		provider = "sparkops"
	}
	return ApprovalSpec{
		Provider:          provider,
		ApproverIDs:       approverIDs,
		RequiredApprovals: required,
		TimeoutMinutes:    timeout,
	}
}

func intFromAny(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
