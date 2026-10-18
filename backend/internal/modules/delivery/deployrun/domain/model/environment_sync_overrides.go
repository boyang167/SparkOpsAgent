package model

import "strings"

func ResourceOverrideKey(kind, name string) string {
	return strings.TrimSpace(kind) + "/" + strings.TrimSpace(name)
}

func ApplyResourceOverrides(resources []EnvironmentSyncResource, overrides map[string]string) []EnvironmentSyncResource {
	if len(overrides) == 0 {
		return resources
	}
	out := make([]EnvironmentSyncResource, len(resources))
	copy(out, resources)
	for i := range out {
		if out[i].ManagedPullSecret {
			continue
		}
		key := ResourceOverrideKey(out[i].Kind, out[i].Name)
		if yaml, ok := overrides[key]; ok && strings.TrimSpace(yaml) != "" {
			out[i].YAML = yaml
		}
	}
	return out
}

func SameResourceOverrides(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
