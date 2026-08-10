package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GitBranchView struct {
	Name      string `json:"name"`
	Default   bool   `json:"default"`
	Protected bool   `json:"protected"`
}

type ListGitBranchesHandler struct {
	checker port.ProjectChecker
	lister  port.GitBranchLister
}

func NewListGitBranchesHandler(checker port.ProjectChecker, lister port.GitBranchLister) *ListGitBranchesHandler {
	return &ListGitBranchesHandler{checker: checker, lister: lister}
}

func (h *ListGitBranchesHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) ([]GitBranchView, error) {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	projectName, err := h.checker.ProjectName(ctx, projectID)
	if err != nil {
		return nil, err
	}
	branches, err := h.lister.ListBranches(ctx, projectName)
	if err != nil {
		return nil, err
	}
	out := make([]GitBranchView, 0, len(branches))
	for _, b := range branches {
		out = append(out, GitBranchView{Name: b.Name, Default: b.Default, Protected: b.Protected})
	}
	return out, nil
}
