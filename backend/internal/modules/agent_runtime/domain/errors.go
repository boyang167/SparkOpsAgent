package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrConversationNotFound = kernel.NewDomainError("RUNTIME_CONVERSATION_NOT_FOUND", "conversation not found")
	ErrRunNotFound          = kernel.NewDomainError("RUNTIME_RUN_NOT_FOUND", "run not found")
	ErrAgentNotPublished    = kernel.NewDomainError("RUNTIME_AGENT_NOT_PUBLISHED", "agent is not published")
	ErrAgentNotFound        = kernel.NewDomainError("RUNTIME_AGENT_NOT_FOUND", "agent not found")
	ErrRunNotRunning        = kernel.NewDomainError("RUNTIME_RUN_NOT_RUNNING", "run is not running")
	ErrAccessDenied         = kernel.NewDomainError("RUNTIME_ACCESS_DENIED", "access denied")
)
