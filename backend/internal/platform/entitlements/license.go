package entitlements

import (
	"strings"
	"time"
)

type licenseConfig struct {
	Edition   string
	ExpiresAt *time.Time
}

func parseLicenseConfig(raw any) licenseConfig {
	lic := licenseConfig{Edition: "community"}
	m, ok := raw.(map[string]any)
	if !ok {
		return lic
	}
	if edition, ok := m["edition"].(string); ok && edition != "" {
		lic.Edition = strings.ToLower(edition)
	}
	if expires, ok := m["expires_at"].(string); ok && expires != "" {
		if t, err := time.Parse(time.RFC3339, expires); err == nil {
			lic.ExpiresAt = &t
		} else if t, err := time.Parse("2006-01-02", expires); err == nil {
			lic.ExpiresAt = &t
		}
	}
	return lic
}

func (l licenseConfig) expired(now time.Time) bool {
	if l.ExpiresAt == nil {
		return false
	}
	return now.After(*l.ExpiresAt)
}

func applyLicense(ents *WorkspaceEntitlements, lic licenseConfig, now time.Time) {
	if lic.expired(now) {
		ents.Agent = true
		ents.Knowledge = true
		ents.Delivery = false
		ents.K8s = false
		ents.Observability = false
		ents.Mcp = false
		return
	}

	switch lic.Edition {
	case "enterprise":
		return
	case "professional":
		ents.Mcp = false
	case "community", "":
		ents.Mcp = false
	default:
		ents.Mcp = false
	}
}

func parseWorkspaceFeatureOverrides(raw any) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	m, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	for wsID, value := range m {
		features, ok := value.(map[string]any)
		if !ok {
			continue
		}
		parsed := make(map[string]bool, len(features))
		for key, val := range features {
			parsed[key] = asBool(val)
		}
		if len(parsed) > 0 {
			out[wsID] = parsed
		}
	}
	return out
}

func applyWorkspaceOverrides(ents *WorkspaceEntitlements, overrides map[string]bool) {
	if len(overrides) == 0 {
		return
	}
	if v, ok := overrides["agent"]; ok {
		ents.Agent = v
	}
	if v, ok := overrides["knowledge"]; ok {
		ents.Knowledge = v
	}
	if v, ok := overrides["delivery"]; ok {
		ents.Delivery = v
	}
	if v, ok := overrides["k8s"]; ok {
		ents.K8s = v
	}
	if v, ok := overrides["observability"]; ok {
		ents.Observability = v
	}
	if v, ok := overrides["mcp"]; ok {
		ents.Mcp = v
	}
}
