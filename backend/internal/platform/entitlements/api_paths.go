package entitlements

import "strings"

// APIPathEntitlement maps an HTTP API path to a module key. Empty means no check.
func APIPathEntitlement(path string) string {
	if !strings.HasPrefix(path, "/api/v1/") {
		return ""
	}
	if strings.HasPrefix(path, "/api/v1/plugins/") {
		rest := strings.TrimPrefix(path, "/api/v1/plugins/")
		if id, suffix, ok := strings.Cut(rest, "/"); ok && id != "" && suffix != "" {
			return "plugin:" + id
		}
	}
	if shouldSkipAPIEntitlement(path) {
		return ""
	}
	if isGovernanceWorkspaceAPI(path) {
		return ""
	}

	switch {
	case strings.Contains(path, "/knowledge-bases"), strings.HasPrefix(path, "/api/v1/documents/"):
		return "knowledge"
	case strings.Contains(path, "/builds"), strings.Contains(path, "/products"),
		strings.Contains(path, "/workspaces/") && strings.Contains(path, "/secrets"):
		return "delivery"
	case strings.Contains(path, "/webhooks"), strings.Contains(path, "/mcp/"):
		return "mcp"
	case strings.Contains(path, "/cronjobs"), strings.Contains(path, "/pods"),
		strings.Contains(path, "/k8s/"),
		strings.Contains(path, "/consul/"), strings.Contains(path, "/deployments"),
		strings.Contains(path, "/ingresses"), strings.Contains(path, "/configmaps"),
		strings.Contains(path, "/services"), strings.Contains(path, "/pvcs"),
		strings.Contains(path, "/virtualservices"), strings.Contains(path, "/storageclasses"):
		return "k8s"
	case strings.Contains(path, "/monitor/"), strings.Contains(path, "/observability/"),
		strings.Contains(path, "/statistics"):
		return "observability"
	case strings.Contains(path, "/projects"), strings.HasPrefix(path, "/api/v1/agents/"),
		strings.HasPrefix(path, "/api/v1/runs/"), strings.HasPrefix(path, "/api/v1/conversations/"),
		strings.HasPrefix(path, "/api/v1/workflows/"), strings.HasPrefix(path, "/api/v1/workflow-runs/"),
		strings.Contains(path, "/templates"):
		return "agent"
	default:
		return ""
	}
}

func shouldSkipAPIEntitlement(path string) bool {
	prefixes := []string{
		"/api/v1/admin/",
		"/api/v1/auth/",
		"/api/v1/public/",
		"/api/v1/platform/",
		"/api/v1/orgs/",
		"/api/v1/notifications",
		"/api/v1/models",
		"/api/v1/identity/",
		"/api/v1/members/",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	if strings.HasSuffix(path, "/entitlements") || strings.HasSuffix(path, "/dashboard") {
		return true
	}
	if path == "/api/v1/workspaces" || strings.HasSuffix(path, "/workspaces/select") {
		return true
	}
	if strings.Contains(path, "/workspaces/") && strings.Contains(path, "/clusters") {
		return true
	}
	if isWorkspacePluginManagementAPI(path) {
		return true
	}
	return false
}

func isWorkspacePluginManagementAPI(path string) bool {
	const prefix = "/api/v1/workspaces/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) == 2 {
		return parts[0] != "" && parts[1] == "plugins"
	}
	if len(parts) == 4 {
		return parts[0] != "" && parts[1] == "plugins" && parts[2] != "" &&
			(parts[3] == "enable" || parts[3] == "disable")
	}
	return false
}

func isGovernanceWorkspaceAPI(path string) bool {
	if !strings.Contains(path, "/workspaces/") {
		return false
	}
	markers := []string{
		"/roles", "/members", "/me/", "/menus", "/apis", "/api-tokens", "/data-resources",
		"/operation-records",
	}
	for _, marker := range markers {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

func (e WorkspaceEntitlements) AllowsModule(key string) bool {
	if key == "" {
		return true
	}
	if strings.HasPrefix(key, "plugin:") {
		id := strings.TrimPrefix(key, "plugin:")
		for _, enabledID := range e.Plugins {
			if id != "" && enabledID == id {
				return true
			}
		}
		return false
	}
	switch key {
	case "agent":
		return e.Agent
	case "knowledge":
		return e.Knowledge
	case "delivery":
		return e.Delivery
	case "k8s":
		return e.K8s
	case "observability":
		return e.Observability
	case "mcp":
		return e.Mcp
	default:
		return true
	}
}

func (e WorkspaceEntitlements) AllowsAPI(path string) bool {
	return e.AllowsModule(APIPathEntitlement(path))
}
