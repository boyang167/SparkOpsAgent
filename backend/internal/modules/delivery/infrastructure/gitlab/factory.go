package gitlab

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
)

type stubLister struct{}

func (stubLister) ListBranches(_ context.Context, _ string) ([]port.GitBranch, error) {
	return []port.GitBranch{
		{Name: "main", Default: true, Protected: true},
		{Name: "develop", Default: false, Protected: false},
	}, nil
}

func NewBranchLister(client *Client) port.GitBranchLister {
	if client == nil {
		return stubLister{}
	}
	return client
}

func NewListGitBranchesHandler(checker port.ProjectChecker, lister port.GitBranchLister) *query.ListGitBranchesHandler {
	return query.NewListGitBranchesHandler(checker, lister)
}
