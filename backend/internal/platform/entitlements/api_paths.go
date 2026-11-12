package entitlements

import (
	"net/url"
	"strings"
)

// APIPathEntitlement maps an HTTP API path to a module key. Empty means no check.
func APIPathEntitlement(path string) string {
	normalized, ok := normalizeAPIPath(path)
	if !ok || !strings.HasPrefix(normalized, "/api/v1/") {
		return ""
	}
	if isWorkspaceFilesystemAPI(normalized) {
		return "filesystem"
	}
	if strings.HasPrefix(normalized, "/api/v1/plugins/") {
		rest := strings.TrimPrefix(normalized, "/api/v1/plugins/")
		if id, suffix, ok := strings.Cut(rest, "/"); ok && id != "" && suffix != "" {
			return "plugin:" + id
		}
	}
	if shouldSkipAPIEntitlement(normalized) {
		return ""
	}
	if isGovernanceWorkspaceAPI(normalized) {
		return ""
	}

	switch {
	case strings.Contains(normalized, "/knowledge-bases"), strings.HasPrefix(normalized, "/api/v1/documents/"):
		return "knowledge"
	case strings.Contains(normalized, "/builds"), strings.Contains(normalized, "/products"),
		strings.Contains(normalized, "/workspaces/") && strings.Contains(normalized, "/secrets"),
		strings.Contains(normalized, "/projects/") && strings.Contains(normalized, "/webhooks"),
		strings.HasPrefix(normalized, "/api/v1/deployments/"),
		strings.Contains(normalized, "/projects/") && strings.Contains(normalized, "/deployments") && !strings.Contains(normalized, "/k8s/"),
		strings.HasPrefix(normalized, "/api/v1/projects/") &&
			(strings.Contains(normalized, "/pods") || strings.Contains(normalized, "/runtime/")):
		return "delivery"
	case strings.Contains(normalized, "/mcp/"):
		return "mcp"
	case strings.Contains(normalized, "/cronjobs"), strings.Contains(normalized, "/pods"),
		strings.Contains(normalized, "/k8s/"),
		strings.Contains(normalized, "/storageclasses"):
		return "k8s"
	case strings.Contains(normalized, "/monitor/"), strings.Contains(normalized, "/observability/"),
		strings.Contains(normalized, "/statistics"):
		return "observability"
	case strings.Contains(normalized, "/projects"), strings.HasPrefix(normalized, "/api/v1/agents/"),
		strings.HasPrefix(normalized, "/api/v1/runs/"), strings.HasPrefix(normalized, "/api/v1/conversations/"),
		strings.HasPrefix(normalized, "/api/v1/workflows/"), strings.HasPrefix(normalized, "/api/v1/workflow-runs/"),
		strings.Contains(normalized, "/templates"):
		return "agent"
	default:
		return ""
	}
}

func normalizeAPIPath(raw string) (string, bool) {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Path == "" {
		return "", false
	}
	if _, err := url.ParseQuery(u.RawQuery); err != nil {
		return "", false
	}
	path, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return "", false
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}
	return path, true
}

func isWorkspaceFilesystemAPI(path string) bool {
	path = strings.TrimSuffix(strings.SplitN(path, "?", 2)[0], "/")
	const prefix = "/api/v1/workspaces/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) < 2 || parts[0] == "" {
		return false
	}
	switch parts[1] {
	case "files", "namespaces", "transfers":
		return true
	default:
		return false
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
	case "filesystem":
		return e.Filesystem
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
