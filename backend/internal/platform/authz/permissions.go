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
