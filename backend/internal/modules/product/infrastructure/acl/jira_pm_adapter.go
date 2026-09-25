package acl

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type JiraPmAdapter struct {
	inner *JiraConnector
}

func NewJiraPmAdapter(inner *JiraConnector) *JiraPmAdapter {
	return &JiraPmAdapter{inner: inner}
}

func (a *JiraPmAdapter) Provider() string { return port.PmProviderJira }

func (a *JiraPmAdapter) GetIterationIssues(ctx context.Context, wsID kernel.WorkspaceID, _, iterationID string, issueTypes []string) ([]port.PmIssue, error) {
	items, err := a.inner.GetSprintIssues(ctx, wsID, iterationID, issueTypes)
	if err != nil {
		return nil, err
	}
	out := make([]port.PmIssue, 0, len(items))
	for _, item := range items {
		out = append(out, pmFromJira(item))
	}
	return out, nil
}

func (a *JiraPmAdapter) GetIssue(ctx context.Context, wsID kernel.WorkspaceID, _, issueKey string) (*port.PmIssue, error) {
	item, err := a.inner.GetIssue(ctx, wsID, issueKey)
	if err != nil {
		return nil, err
	}
	out := pmFromJira(*item)
	return &out, nil
}

func (a *JiraPmAdapter) TransitionIssue(ctx context.Context, wsID kernel.WorkspaceID, _, issueKey, targetStatus string) error {
	return a.inner.TransitionIssue(ctx, wsID, issueKey, targetStatus)
}

func pmFromJira(item port.JiraIssue) port.PmIssue {
	return port.PmIssue{
		Key: item.Key, ID: item.ID, Summary: item.Summary, Description: item.Description,
		IssueType: item.IssueType, Status: item.Status, Priority: item.Priority, URL: item.URL,
	}
}
