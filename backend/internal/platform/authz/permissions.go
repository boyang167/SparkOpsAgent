package authz

const (
	PermProjectRead     = "project:read"
	PermProjectWrite    = "project:write"
	PermProjectSubmit   = "project:submit"
	PermProjectReview   = "project:review"
	PermProjectOperate  = "project:operate"
	PermSecretRead      = "secret:read"
	PermSecretWrite     = "secret:write"
	PermAgentRead       = "agent:read"
	PermAgentWrite      = "agent:write"
	PermAgentPublish    = "agent:publish"
	PermChatUse         = "chat:use"
	PermGovernanceAdmin = "governance:admin"
	PermPlatformAdmin   = "platform:admin"
	PermFileRead        = "file:read"
	PermFileWrite       = "file:write"
	PermFileDelete      = "file:delete"
	PermFileShare       = "file:share"
	PermFileAdmin       = "file:admin"
	PermStorageRead     = "storage:read"
	PermStorageAdmin    = "storage:admin"
	PermLifecycleAdmin  = "lifecycle:admin"
)

var DefaultAdminPermissions = []string{
	PermProjectRead,
	PermProjectWrite,
	PermProjectSubmit,
	PermProjectReview,
	PermProjectOperate,
	PermSecretRead,
	PermSecretWrite,
	PermAgentRead,
	PermAgentWrite,
	PermAgentPublish,
	PermChatUse,
	PermGovernanceAdmin,
	PermFileRead,
	PermFileWrite,
	PermFileDelete,
	PermFileShare,
	PermFileAdmin,
	PermStorageRead,
	PermStorageAdmin,
	PermLifecycleAdmin,
}

var defaultMemberFilesystemPermissions = []string{
	PermFileRead,
	PermFileWrite,
	PermFileDelete,
}

func RequiredDefaultRolePermissions(name string) []string {
	switch name {
	case "admin":
		return DefaultAdminPermissions
	case "developer", "operator":
		return defaultMemberFilesystemPermissions
	default:
		return nil
	}
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
	PermProjectSubmit,
	PermProjectReview,
	PermProjectOperate,
	PermSecretRead,
	PermSecretWrite,
	PermAgentRead,
	PermAgentWrite,
	PermAgentPublish,
	PermChatUse,
	PermGovernanceAdmin,
	PermFileRead,
	PermFileWrite,
	PermFileDelete,
	PermFileShare,
	PermFileAdmin,
	PermStorageRead,
	PermStorageAdmin,
	PermLifecycleAdmin,
}

type DefaultRoleDefinition struct {
	Name        string
	Permissions []string
}

var DefaultWorkspaceRoles = []DefaultRoleDefinition{
	{Name: "admin", Permissions: DefaultAdminPermissions},
	{Name: "developer", Permissions: []string{
		PermProjectRead, PermProjectWrite, PermProjectSubmit,
		PermSecretRead, PermAgentRead, PermChatUse, PermFileRead, PermFileWrite, PermFileDelete,
	}},
	{Name: "reviewer", Permissions: []string{
		PermProjectRead, PermProjectReview, PermSecretRead,
	}},
	{Name: "operator", Permissions: []string{
		PermProjectRead, PermProjectOperate, PermSecretRead,
		PermFileRead, PermFileWrite, PermFileDelete,
	}},
	{Name: "viewer", Permissions: []string{PermProjectRead}},
}
