package model

import (
	"strings"
)

const MaskedParameterValue = "******"

var reservedJobParameterKeys = map[string]struct{}{"branch": {}, "environment_id": {}}

// IsJobInputOverrideKey reports whether a job parameter key carries a run variable override.
func IsJobInputOverrideKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "_") {
		return false
	}
	_, reserved := reservedJobParameterKeys[key]
	return !reserved
}

func NormalizeParameters(params map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range params {
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

// RestoreMaskedParameters keeps the stored value wherever the client echoed the mask back.
func RestoreMaskedParameters(next, prev map[string]string) map[string]string {
	out := make(map[string]string, len(next))
	for key, value := range next {
		if value == MaskedParameterValue {
			old, ok := prev[key]
			if !ok {
				continue
			}
			value = old
		}
		out[key] = value
	}
	return out
}

func MaskParameters(params map[string]string, secret map[string]struct{}) map[string]string {
	out := make(map[string]string, len(params))
	for key, value := range params {
		if _, ok := secret[key]; ok && value != "" {
			value = MaskedParameterValue
		}
		out[key] = value
	}
	return out
}

// ResolveInput returns the value a job runs with: its own override first, then the plan parameter.
func (spec WorkflowRefSpec) ResolveInput(planParams map[string]string, key string) string {
	if IsJobInputOverrideKey(key) {
		if value := strings.TrimSpace(spec.Parameters[key]); value != "" {
			return value
		}
	}
	return strings.TrimSpace(planParams[key])
}

func (spec WorkflowRefSpec) MissingInputs(planParams map[string]string, required []string) []string {
	missing := []string{}
	for _, key := range required {
		if spec.ResolveInput(planParams, key) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}
