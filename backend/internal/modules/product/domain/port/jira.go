package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type JiraIssue struct {
	Key         string
	ID          string
	Summary     string
	Description string
	IssueType   string
	Status      string
	Priority    string
	URL         string
}

type JiraConnector interface {
	GetSprintIssues(ctx context.Context, wsID kernel.WorkspaceID, jiraSprintID string, issueTypes []string) ([]JiraIssue, error)
	GetIssue(ctx context.Context, wsID kernel.WorkspaceID, issueKey string) (*JiraIssue, error)
	TransitionIssue(ctx context.Context, wsID kernel.WorkspaceID, issueKey, targetStatus string) error
}
