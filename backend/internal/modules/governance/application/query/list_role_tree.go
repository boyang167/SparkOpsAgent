package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RoleTreeView struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Permissions []string       `json:"permissions"`
	ParentID    *string        `json:"parent_id,omitempty"`
	Children    []RoleTreeView `json:"children,omitempty"`
}

type ListRoleTreeHandler struct {
	repo repository.RoleRepository
}

func NewListRoleTreeHandler(repo repository.RoleRepository) *ListRoleTreeHandler {
	return &ListRoleTreeHandler{repo: repo}
}

func (h *ListRoleTreeHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID) ([]RoleTreeView, error) {
	roles, err := h.repo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return buildRoleTree(roles), nil
}

func buildRoleTree(roles []*model.Role) []RoleTreeView {
	type node struct {
		view RoleTreeView
		kids []*node
	}
	index := make(map[string]*node, len(roles))
	for _, role := range roles {
		view := RoleTreeView{
			ID: role.ID.String(), Name: role.Name, Permissions: role.Permissions,
		}
		if role.ParentID != nil {
			pid := role.ParentID.String()
			view.ParentID = &pid
		}
		index[view.ID] = &node{view: view}
	}
	roots := make([]*node, 0)
	for _, role := range roles {
		current := index[role.ID.String()]
		if role.ParentID == nil {
			roots = append(roots, current)
			continue
		}
		if parent, ok := index[role.ParentID.String()]; ok {
			parent.kids = append(parent.kids, current)
		} else {
			roots = append(roots, current)
		}
	}
	var convert func(*node) RoleTreeView
	convert = func(n *node) RoleTreeView {
		out := n.view
		for _, child := range n.kids {
			out.Children = append(out.Children, convert(child))
		}
		return out
	}
	result := make([]RoleTreeView, 0, len(roots))
	for _, root := range roots {
		result = append(result, convert(root))
	}
	return result
}
