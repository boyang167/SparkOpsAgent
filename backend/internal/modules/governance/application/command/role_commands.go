package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateRoleHandler struct {
	repo repository.RoleRepository
}

func NewCreateRoleHandler(repo repository.RoleRepository) *CreateRoleHandler {
	return &CreateRoleHandler{repo: repo}
}

func (h *CreateRoleHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, name string, permissions []string, parentID *kernel.RoleID) (*model.Role, error) {
	if parentID != nil {
		parent, err := h.repo.FindByID(ctx, *parentID)
		if err != nil {
			return nil, err
		}
		if parent.WorkspaceID != workspaceID {
			return nil, domain.ErrInvalidParentRole
		}
	}
	exists, err := h.repo.ExistsByName(ctx, workspaceID, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrRoleAlreadyExists
	}
	role := model.NewRole(workspaceID, name, permissions, parentID)
	if err := h.repo.Save(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

type AssignRoleHandler struct {
	roles    repository.RoleRepository
	bindings repository.BindingRepository
	sync     *service.APIPolicySync
}

func NewAssignRoleHandler(
	roles repository.RoleRepository,
	bindings repository.BindingRepository,
	sync *service.APIPolicySync,
) *AssignRoleHandler {
	return &AssignRoleHandler{roles: roles, bindings: bindings, sync: sync}
}

func (h *AssignRoleHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, roleID kernel.RoleID, memberID kernel.MemberID) error {
	role, err := h.roles.FindByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role.WorkspaceID != workspaceID {
		return domain.ErrRoleNotFound
	}
	exists, err := h.bindings.Exists(ctx, roleID, memberID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := h.bindings.Save(ctx, model.NewRoleBinding(roleID, memberID)); err != nil {
		return err
	}
	if h.sync != nil {
		return h.sync.SyncMemberRole(ctx, workspaceID, roleID, memberID, true)
	}
	return nil
}

type UpdateRoleHandler struct {
	repo repository.RoleRepository
}

func NewUpdateRoleHandler(repo repository.RoleRepository) *UpdateRoleHandler {
	return &UpdateRoleHandler{repo: repo}
}

type UpdateRoleInput struct {
	Name              *string
	ParentID          *kernel.RoleID
	UpdateParent      bool
	Permissions       []string
	UpdatePermissions bool
}

func (h *UpdateRoleHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	roleID kernel.RoleID,
	input UpdateRoleInput,
) (*model.Role, error) {
	role, err := h.repo.FindByID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if role.WorkspaceID != workspaceID {
		return nil, domain.ErrRoleNotFound
	}
	if !input.UpdatePermissions && !input.UpdateParent && input.Name == nil {
		return nil, domain.ErrInvalidRoleUpdate
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, domain.ErrInvalidRoleUpdate
		}
		if name != role.Name {
			exists, err := h.repo.ExistsByName(ctx, workspaceID, name)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, domain.ErrRoleAlreadyExists
			}
			role.Name = name
		}
	}
	if input.UpdateParent {
		if input.ParentID != nil {
			if *input.ParentID == roleID {
				return nil, domain.ErrInvalidParentRole
			}
			parent, err := h.repo.FindByID(ctx, *input.ParentID)
			if err != nil {
				return nil, err
			}
			if parent.WorkspaceID != workspaceID {
				return nil, domain.ErrInvalidParentRole
			}
		}
		role.ParentID = input.ParentID
	}
	if input.UpdatePermissions {
		if len(input.Permissions) == 0 {
			return nil, domain.ErrPermissionsRequired
		}
		role.UpdatePermissions(input.Permissions)
	}
	if err := h.repo.Save(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

type DeleteRoleHandler struct {
	roles      repository.RoleRepository
	bindings   repository.BindingRepository
	apiStore   *persistence.RoleAPIPolicyStore
	dataStore  *persistence.DataPolicyStore
	sync       *service.APIPolicySync
}

func NewDeleteRoleHandler(
	roles repository.RoleRepository,
	bindings repository.BindingRepository,
	apiStore *persistence.RoleAPIPolicyStore,
	dataStore *persistence.DataPolicyStore,
	sync *service.APIPolicySync,
) *DeleteRoleHandler {
	return &DeleteRoleHandler{
		roles: roles, bindings: bindings, apiStore: apiStore, dataStore: dataStore, sync: sync,
	}
}

func (h *DeleteRoleHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, roleID kernel.RoleID) error {
	role, err := h.roles.FindByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role.WorkspaceID != workspaceID {
		return domain.ErrRoleNotFound
	}
	if role.IsProtected() {
		return domain.ErrRoleProtected
	}
	count, err := h.roles.CountChildren(ctx, roleID)
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrRoleHasChildren
	}
	if err := h.bindings.DeleteByRoleID(ctx, roleID); err != nil {
		return err
	}
	if h.apiStore != nil {
		_ = h.apiStore.DeleteByRole(ctx, roleID.String())
	}
	if h.dataStore != nil {
		_ = h.dataStore.DeleteByRole(ctx, roleID.String())
	}
	if h.sync != nil {
		if err := h.sync.RemoveRolePolicies(ctx, roleID, workspaceID); err != nil {
			return err
		}
	}
	return h.roles.Delete(ctx, roleID)
}

type BootstrapWorkspaceHandler struct {
	roles    repository.RoleRepository
	bindings repository.BindingRepository
	sync     *service.APIPolicySync
}

func NewBootstrapWorkspaceHandler(
	roles repository.RoleRepository,
	bindings repository.BindingRepository,
	sync *service.APIPolicySync,
) *BootstrapWorkspaceHandler {
	return &BootstrapWorkspaceHandler{roles: roles, bindings: bindings, sync: sync}
}

func (h *BootstrapWorkspaceHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, ownerID kernel.MemberID, permissions []string) error {
	exists, err := h.roles.ExistsByName(ctx, workspaceID, "admin")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	role := model.NewRole(workspaceID, "admin", permissions, nil)
	if err := h.roles.Save(ctx, role); err != nil {
		return err
	}
	if err := h.bindings.Save(ctx, model.NewRoleBinding(role.ID, ownerID)); err != nil {
		return err
	}
	if h.sync != nil {
		return h.sync.SyncMemberRole(ctx, workspaceID, role.ID, ownerID, true)
	}
	return nil
}

type CopyRoleHandler struct {
	repo      repository.RoleRepository
	apiStore  *persistence.RoleAPIPolicyStore
	menuStore *persistence.MenuStore
	dataStore *persistence.DataPolicyStore
	sync      *service.APIPolicySync
}

func NewCopyRoleHandler(
	repo repository.RoleRepository,
	apiStore *persistence.RoleAPIPolicyStore,
	menuStore *persistence.MenuStore,
	dataStore *persistence.DataPolicyStore,
	sync *service.APIPolicySync,
) *CopyRoleHandler {
	return &CopyRoleHandler{
		repo: repo, apiStore: apiStore, menuStore: menuStore, dataStore: dataStore, sync: sync,
	}
}

func (h *CopyRoleHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, roleID kernel.RoleID, name string) (*model.Role, error) {
	source, err := h.repo.FindByID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if source.WorkspaceID != workspaceID {
		return nil, domain.ErrRoleNotFound
	}
	if name == "" {
		name = source.Name + "_copy"
	}
	exists, err := h.repo.ExistsByName(ctx, workspaceID, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrRoleAlreadyExists
	}
	copyRole := model.NewRole(workspaceID, name, source.Permissions, source.ParentID)
	if err := h.repo.Save(ctx, copyRole); err != nil {
		return nil, err
	}

	sourceID := roleID.String()
	copyID := copyRole.ID.String()

	if h.apiStore != nil {
		apiPolicies, err := h.apiStore.ListByRole(ctx, sourceID)
		if err != nil {
			return nil, err
		}
		if err := h.apiStore.ReplaceForRole(ctx, copyID, apiPolicies); err != nil {
			return nil, err
		}
	}

	if h.menuStore != nil {
		menuIDs, err := h.menuStore.ListMenuIDsByRole(ctx, workspaceID, sourceID)
		if err != nil {
			return nil, err
		}
		if err := h.menuStore.SetMenuIDsForRole(ctx, workspaceID, copyID, menuIDs); err != nil {
			return nil, err
		}
	}

	if h.dataStore != nil {
		dataPolicies, err := h.dataStore.ListByRole(ctx, sourceID)
		if err != nil {
			return nil, err
		}
		if err := h.dataStore.ReplaceForRole(ctx, copyID, dataPolicies); err != nil {
			return nil, err
		}
	}

	if h.sync != nil {
		if err := h.sync.SyncRolePolicies(ctx, copyRole.ID, workspaceID); err != nil {
			return nil, err
		}
	}

	return copyRole, nil
}
