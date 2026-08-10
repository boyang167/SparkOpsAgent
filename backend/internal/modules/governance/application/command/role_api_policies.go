package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UpdateRoleAPIPoliciesHandler struct {
	roles repository.RoleRepository
	store *persistence.RoleAPIPolicyStore
	sync  *service.APIPolicySync
}

func NewUpdateRoleAPIPoliciesHandler(
	roles repository.RoleRepository,
	store *persistence.RoleAPIPolicyStore,
	sync *service.APIPolicySync,
) *UpdateRoleAPIPoliciesHandler {
	return &UpdateRoleAPIPoliciesHandler{roles: roles, store: store, sync: sync}
}

func (h *UpdateRoleAPIPoliciesHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	roleID kernel.RoleID,
	policies []persistence.RoleAPIPolicy,
) error {
	role, err := h.roles.FindByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role.WorkspaceID != wsID {
		return domain.ErrRoleNotFound
	}
	if err := h.store.ReplaceForRole(ctx, roleID.String(), policies); err != nil {
		return err
	}
	if h.sync != nil {
		return h.sync.SyncRolePolicies(ctx, roleID, wsID)
	}
	return nil
}
