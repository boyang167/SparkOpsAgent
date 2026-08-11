package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UpdateRoleDataPoliciesHandler struct {
	roles repository.RoleRepository
	store *persistence.DataPolicyStore
}

func NewUpdateRoleDataPoliciesHandler(
	roles repository.RoleRepository,
	store *persistence.DataPolicyStore,
) *UpdateRoleDataPoliciesHandler {
	return &UpdateRoleDataPoliciesHandler{roles: roles, store: store}
}

func (h *UpdateRoleDataPoliciesHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	roleID kernel.RoleID,
	policies []persistence.RoleDataPolicy,
) error {
	role, err := h.roles.FindByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role.WorkspaceID != wsID {
		return domain.ErrRoleNotFound
	}
	return h.store.ReplaceForRole(ctx, roleID.String(), policies)
}
