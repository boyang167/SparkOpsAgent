package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListRoleDataPoliciesHandler struct {
	roles repository.RoleRepository
	store *persistence.DataPolicyStore
}

func NewListRoleDataPoliciesHandler(roles repository.RoleRepository, store *persistence.DataPolicyStore) *ListRoleDataPoliciesHandler {
	return &ListRoleDataPoliciesHandler{roles: roles, store: store}
}

func (h *ListRoleDataPoliciesHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	roleID kernel.RoleID,
) ([]persistence.RoleDataPolicy, error) {
	role, err := h.roles.FindByID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if role.WorkspaceID != wsID {
		return nil, domain.ErrRoleNotFound
	}
	return h.store.ListByRole(ctx, roleID.String())
}
