package authz

const (
	PermProjectRead   = "project:read"
	PermProjectWrite  = "project:write"
	PermSecretRead    = "secret:read"
	PermSecretWrite   = "secret:write"
	PermAgentRead     = "agent:read"
	PermAgentWrite    = "agent:write"
	PermAgentPublish  = "agent:publish"
	PermChatUse       = "chat:use"
	PermGovernanceAdmin = "governance:admin"
	PermPlatformAdmin   = "platform:admin"
)

var DefaultAdminPermissions = []string{
	PermProjectRead,
	PermProjectWrite,
	PermSecretRead,
	PermSecretWrite,
	PermAgentRead,
	PermAgentWrite,
	PermAgentPublish,
	PermChatUse,
	PermGovernanceAdmin,
}

// MergePermissions returns existing permissions plus any defaults not yet present.
func MergePermissions(existing, defaults []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(defaults))
	merged := make([]string, 0, len(existing)+len(defaults))
	for _, perm := range existing {
		if perm == "" {
			continue
		}
		if _, ok := seen[perm]; ok {
			continue
		}
		seen[perm] = struct{}{}
		merged = append(merged, perm)
	}
	for _, perm := range defaults {
		if perm == "" {
			continue
		}
		if _, ok := seen[perm]; ok {
			continue
		}
		seen[perm] = struct{}{}
		merged = append(merged, perm)
	}
	return merged
}

// AllPermissions lists assignable permissions for custom workspace roles.
var AllPermissions = []string{
	PermProjectRead,
	PermProjectWrite,
	PermSecretRead,
	PermSecretWrite,
	PermAgentRead,
	PermAgentWrite,
	PermAgentPublish,
	PermChatUse,
	PermGovernanceAdmin,
}
