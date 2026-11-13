package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type LifecycleAction string

const (
	LifecycleActionReplicate LifecycleAction = "replicate"
	LifecycleActionArchive   LifecycleAction = "archive"
	LifecycleActionTrash     LifecycleAction = "trash"
	LifecycleActionPurge     LifecycleAction = "purge"
	LifecycleActionRescan    LifecycleAction = "rescan"
)

type LifecycleCandidate struct {
	OrganizationID   uuid.UUID
	ScopeType        model.ScopeType
	ScopeID          uuid.UUID
	Classification   model.TransferClassification
	MIMEType         string
	Size             int64
	CreatedAt        time.Time
	LastAccessedAt   time.Time
	RetentionUntil   *time.Time
	LegalHold        bool
}

type LifecycleDecision struct {
	RuleID  string
	Actions []LifecycleAction
}

type LifecycleEngine struct{}

func NewLifecycleEngine() *LifecycleEngine { return &LifecycleEngine{} }

func (e *LifecycleEngine) Evaluate(policy *model.LifecyclePolicy, candidate LifecycleCandidate) ([]LifecycleDecision, error) {
	if policy == nil || policy.Status != model.LifecyclePolicyActive {
		return nil, nil
	}
	rawRules, ok := policy.Rules["rules"].([]any)
	if !ok {
		return nil, nil
	}
	now := time.Now().UTC()
	var decisions []LifecycleDecision
	for _, raw := range rawRules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		when, _ := rule["when"].(map[string]any)
		if !matchesLifecycleWhen(when, candidate, now) {
			continue
		}
		ruleID, _ := rule["id"].(string)
		if strings.TrimSpace(ruleID) == "" {
			ruleID = fmt.Sprintf("rule-%d", len(decisions)+1)
		}
		actions := parseLifecycleActions(rule["actions"])
		if len(actions) == 0 {
			continue
		}
		if candidate.LegalHold && containsAction(actions, LifecycleActionPurge) {
			continue
		}
		decisions = append(decisions, LifecycleDecision{RuleID: ruleID, Actions: actions})
	}
	return decisions, nil
}

func matchesLifecycleWhen(when map[string]any, candidate LifecycleCandidate, now time.Time) bool {
	if when == nil {
		return true
	}
	if scope, ok := when["scope"].(string); ok && strings.TrimSpace(scope) != "" {
		want := model.ScopeType(strings.ToUpper(strings.TrimSpace(scope)))
		if want != candidate.ScopeType {
			return false
		}
	}
	if classification, ok := when["classification"].(string); ok && strings.TrimSpace(classification) != "" {
		if strings.ToUpper(classification) != string(candidate.Classification) {
			return false
		}
	}
	if mimePrefix, ok := when["mime_prefix"].(string); ok && mimePrefix != "" {
		if !strings.HasPrefix(strings.ToLower(candidate.MIMEType), strings.ToLower(mimePrefix)) {
			return false
		}
	}
	if minSize, ok := numericAsInt64(when["min_size"]); ok && candidate.Size < minSize {
		return false
	}
	if maxSize, ok := numericAsInt64(when["max_size"]); ok && candidate.Size > maxSize {
		return false
	}
	if minAgeHours, ok := numericAsFloat64(when["min_age_hours"]); ok {
		age := now.Sub(candidate.CreatedAt)
		if age < time.Duration(minAgeHours*float64(time.Hour)) {
			return false
		}
	}
	if maxAgeHours, ok := numericAsFloat64(when["max_age_hours"]); ok {
		age := now.Sub(candidate.CreatedAt)
		if age < time.Duration(maxAgeHours*float64(time.Hour)) {
			return false
		}
	}
	if maxIdleHours, ok := numericAsFloat64(when["max_idle_hours"]); ok && !candidate.LastAccessedAt.IsZero() {
		idle := now.Sub(candidate.LastAccessedAt)
		if idle < time.Duration(maxIdleHours*float64(time.Hour)) {
			return false
		}
	}
	if retentionRequired, ok := when["retention"].(bool); ok && retentionRequired {
		if candidate.RetentionUntil == nil || candidate.RetentionUntil.After(now) {
			return false
		}
	}
	if legalHold, ok := when["legal_hold"].(bool); ok && legalHold != candidate.LegalHold {
		return false
	}
	return true
}

func parseLifecycleActions(raw any) []LifecycleAction {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]LifecycleAction, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			continue
		}
		switch LifecycleAction(strings.ToLower(strings.TrimSpace(value))) {
		case LifecycleActionReplicate, LifecycleActionArchive, LifecycleActionTrash,
			LifecycleActionPurge, LifecycleActionRescan:
			out = append(out, LifecycleAction(strings.ToLower(strings.TrimSpace(value))))
		}
	}
	return out
}

func containsAction(actions []LifecycleAction, want LifecycleAction) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

func numericAsInt64(raw any) (int64, bool) {
	switch value := raw.(type) {
	case float64:
		return int64(value), true
	case int64:
		return value, true
	case int:
		return int64(value), true
	default:
		return 0, false
	}
}

func numericAsFloat64(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case int64:
		return float64(value), true
	case int:
		return float64(value), true
	default:
		return 0, false
	}
}
