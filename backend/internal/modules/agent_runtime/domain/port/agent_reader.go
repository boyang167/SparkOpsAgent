package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PublishedAgentVersion struct {
	AgentID          kernel.AgentID
	VersionID        kernel.AgentVersionID
	ProjectID        kernel.ProjectID
	SystemPrompt     string
	ModelName        string
	Temperature      float64
	MaxTokens        int
	KnowledgeBaseIDs []string
}

type AgentReader interface {
	GetPublishedVersion(ctx context.Context, agentID kernel.AgentID) (*PublishedAgentVersion, error)
	AgentInWorkspace(ctx context.Context, agentID kernel.AgentID, wsID kernel.WorkspaceID) (bool, error)
}
