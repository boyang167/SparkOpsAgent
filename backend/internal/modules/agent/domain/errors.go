package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidAgentName     = kernel.NewDomainError("AGENT_INVALID_NAME", "invalid agent name")
	ErrAgentNotFound        = kernel.NewDomainError("AGENT_NOT_FOUND", "agent not found")
	ErrAgentNameExists      = kernel.NewDomainError("AGENT_NAME_EXISTS", "agent name already exists")
	ErrAgentArchived        = kernel.NewDomainError("AGENT_ARCHIVED", "agent is archived")
	ErrAgentNotPublished    = kernel.NewDomainError("AGENT_NOT_PUBLISHED", "agent is not published")
	ErrVersionNotFound      = kernel.NewDomainError("AGENT_VERSION_NOT_FOUND", "agent version not found")
	ErrProjectAccessDenied  = kernel.NewDomainError("AGENT_PROJECT_ACCESS_DENIED", "project access denied")
	ErrProjectNotActive     = kernel.NewDomainError("AGENT_PROJECT_NOT_ACTIVE", "project is not active")
)
