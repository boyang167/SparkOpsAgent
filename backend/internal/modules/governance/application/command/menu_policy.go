package command

// DefaultMenuRolePolicies maps menu path → workspace role names allowed to see the menu.
// Once bound, a menu is visible only to listed roles (see FilterMenusForNav).
// Admin is included on every entry so administrators retain full navigation.
//
// Parent group nodes (e.g. "application") are derived automatically when a child is visible.
var DefaultMenuRolePolicies = map[string][]string{
	"/projects":              {"admin", "developer", "operator", "reviewer", "viewer"},
	"/templates":             {"admin", "developer"},
	"/knowledge":             {"admin", "developer"},
	"/files":                 {"admin", "developer", "operator"},
	"/files/data":            {"admin", "developer", "operator"},
	"/files/control":         {"admin", "developer", "operator"},
	"/files/governance":      {"admin", "developer", "operator"},
	"/delivery/builds":       {"admin", "developer", "operator", "reviewer"},
	"/publish/products":      {"admin", "operator"},
	"/products/workflows":    {"admin", "operator"},
	"/publish/release-plans": {"admin", "operator", "reviewer"},
	"/publish/plans":         {"admin", "operator"},
	"/publish/releases":      {"admin", "operator", "reviewer"},
	"/config/secrets":        {"admin", "operator"},
	"/cluster/list":          {"admin", "developer", "operator"},
	"/runtime/pods":          {"admin", "developer", "operator"},
	"/cronjobs":              {"admin", "operator"},
	"/monitor/overview":      {"admin", "developer", "operator", "reviewer", "viewer"},
	"/monitor/log-viewer":    {"admin", "developer", "operator", "reviewer", "viewer"},
	"/monitor/statistics":    {"admin", "operator", "reviewer", "viewer"},
	"/config/webhooks":       {"admin", "operator"},
	"/config/mcp":            {"admin", "operator"},
	// Kubernetes plugin (skipped when plugin menus are not synced yet)
	"/plugins/kubernetes/clusters":        {"admin", "developer", "operator", "reviewer", "viewer"},
	"/plugins/kubernetes/nodes":           {"admin", "operator"},
	"/plugins/kubernetes/workloads":       {"admin", "developer", "operator"},
	"/plugins/kubernetes/namespaces":      {"admin", "developer", "operator"},
	"/plugins/kubernetes/networks":        {"admin", "operator"},
	"/plugins/kubernetes/gateway":         {"admin", "operator"},
	"/plugins/kubernetes/configs":         {"admin", "developer", "operator"},
	"/plugins/kubernetes/storages":        {"admin", "operator"},
	"/plugins/kubernetes/access-controls": {"admin", "operator"},
	"/plugins/kubernetes/records":         {"admin", "operator", "viewer"},
	"/plugins/kubernetes/ops-tools":       {"admin", "developer", "operator"},
	"/plugins/kubernetes/diagnosis":       {"admin", "operator"},
	"/plugins/kubernetes/backup":          {"admin", "operator"},
	"/plugins/kubernetes/chat":            {"admin", "developer", "operator"},
	"/plugins/kubernetes/cloudtty":        {"admin", "developer", "operator"},
	"/plugins/kubernetes/openkruise":      {"admin", "operator"},
	"/plugins/kubernetes/kruise-rollouts": {"admin", "operator"},
	"/plugins/kubernetes/kubeblocks":      {"admin", "operator"},
	"/plugins/kubernetes/kubevela":        {"admin", "operator"},
}
