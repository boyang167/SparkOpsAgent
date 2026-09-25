package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	PmProviderJira      = "jira"
	PmProviderTapd      = "tapd"
	PmProviderPingCode  = "pingcode"
	PmProviderFeishu    = "feishu"
)

type PmIssue struct {
	Key         string
	ID          string
	Summary     string
	Description string
	IssueType   string
	Status      string
	Priority    string
	URL         string
}

type PmConnector interface {
	Provider() string
	GetIterationIssues(ctx context.Context, wsID kernel.WorkspaceID, pmWorkspaceID, iterationID string, issueTypes []string) ([]PmIssue, error)
	GetIssue(ctx context.Context, wsID kernel.WorkspaceID, pmWorkspaceID, issueKey string) (*PmIssue, error)
	TransitionIssue(ctx context.Context, wsID kernel.WorkspaceID, pmWorkspaceID, issueKey, targetStatus string) error
}

type PmConnectorRouter struct {
	connectors map[string]PmConnector
}

func NewPmConnectorRouter(items ...PmConnector) *PmConnectorRouter {
	m := map[string]PmConnector{}
	for _, item := range items {
		if item != nil {
			m[item.Provider()] = item
		}
	}
	return &PmConnectorRouter{connectors: m}
}

func (r *PmConnectorRouter) Get(provider string) PmConnector {
	if r == nil {
		return nil
	}
	return r.connectors[provider]
}
