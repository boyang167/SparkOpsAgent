package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RoleView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type ListRolesHandler struct {
	repo repository.RoleRepository
}

func NewListRolesHandler(repo repository.RoleRepository) *ListRolesHandler {
	return &ListRolesHandler{repo: repo}
}

func (h *ListRolesHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID) ([]RoleView, error) {
	roles, err := h.repo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]RoleView, 0, len(roles))
	for _, role := range roles {
		out = append(out, RoleView{
			ID: role.ID.String(), Name: role.Name, Permissions: role.Permissions,
		})
	}
	return out, nil
}
