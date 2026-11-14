package command

import (
	"context"
	"errors"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	governanceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
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
			if role.IsProtected() {
				return nil, domain.ErrRoleProtected
			}
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
	roles     repository.RoleRepository
	bindings  repository.BindingRepository
	apiStore  *persistence.RoleAPIPolicyStore
	dataStore *persistence.DataPolicyStore
	sync      *service.APIPolicySync
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
	roles      repository.RoleRepository
	bindings   repository.BindingRepository
	sync       *service.APIPolicySync
	operations messaging.OperationWriter
}

func NewBootstrapWorkspaceHandler(
	roles repository.RoleRepository,
	bindings repository.BindingRepository,
	sync *service.APIPolicySync,
	writers ...messaging.OperationWriter,
) *BootstrapWorkspaceHandler {
	handler := &BootstrapWorkspaceHandler{roles: roles, bindings: bindings, sync: sync}
	if len(writers) > 0 {
		handler.operations = writers[0]
	}
	return handler
}

func (h *BootstrapWorkspaceHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, ownerID kernel.MemberID, permissions []string) error {
	roles, err := h.roles.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	byName := make(map[string]*model.Role, len(roles))
	for _, role := range roles {
		byName[role.Name] = role
	}
	var admin *model.Role
	for _, definition := range authz.DefaultWorkspaceRoles {
		role := byName[definition.Name]
		if role == nil {
			role = model.NewRole(workspaceID, definition.Name, definition.Permissions, nil)
			if err := h.roles.Save(ctx, role); err != nil {
				return err
			}
		} else if definition.Name == "admin" {
			merged := authz.MergePermissions(workspaceScopedPermissions(role.Permissions), workspaceScopedPermissions(permissions))
			if !samePermissions(merged, role.Permissions) {
				role.UpdatePermissions(merged)
				if err := h.roles.Save(ctx, role); err != nil {
					return err
				}
			}
		}
		if definition.Name == "admin" {
			admin = role
		}
	}
	if admin == nil {
		return nil
	}
	exists, err := h.bindings.Exists(ctx, admin.ID, ownerID)
	if err != nil {
		return err
	}
	if !exists {
		if err := h.bindings.Save(ctx, model.NewRoleBinding(admin.ID, ownerID)); err != nil {
			return err
		}
	}
	if messaging.IsInboxConsumerContext(ctx) {
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok || h.operations == nil {
			return errors.New("governance policy operation writer is required")
		}
		return h.operations.Schedule(ctx, messaging.OperationRequest{
			Owner: metadata.Subscriber, OperationType: governanceoutbound.OperationTypeMemberRoleSync,
			IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + governanceoutbound.OperationTypeMemberRoleSync + ":" + ownerID.String(),
			Payload: governanceoutbound.MemberRoleSyncPayload{
				EventID: metadata.EventID, WorkspaceID: workspaceID.String(), RoleID: admin.ID.String(),
				MemberID: ownerID.String(), Add: true,
			},
		})
	}
	if h.sync != nil {
		return h.sync.SyncMemberRole(ctx, workspaceID, admin.ID, ownerID, true)
	}
	return nil
}

type EnsureDefaultWorkspaceRolesHandler struct {
	roles repository.RoleRepository
}

func NewEnsureDefaultWorkspaceRolesHandler(roles repository.RoleRepository) *EnsureDefaultWorkspaceRolesHandler {
	return &EnsureDefaultWorkspaceRolesHandler{roles: roles}
}

func (h *EnsureDefaultWorkspaceRolesHandler) Handle(ctx context.Context) error {
	type workspaceLister interface {
		ListWorkspaceIDs(context.Context) ([]kernel.WorkspaceID, error)
	}
	lister, ok := h.roles.(workspaceLister)
	if !ok {
		return nil
	}
	workspaceIDs, err := lister.ListWorkspaceIDs(ctx)
	if err != nil {
		return err
	}
	for _, workspaceID := range workspaceIDs {
		roles, err := h.roles.ListByWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}
		byName := make(map[string]*model.Role, len(roles))
		for _, role := range roles {
			byName[role.Name] = role
		}
		for _, definition := range authz.DefaultWorkspaceRoles {
			role := byName[definition.Name]
			if role == nil {
				if err := h.roles.Save(ctx, model.NewRole(workspaceID, definition.Name, definition.Permissions, nil)); err != nil {
					return err
				}
				continue
			}
			required := authz.RequiredDefaultRolePermissions(definition.Name)
			merged := authz.MergePermissions(workspaceScopedPermissions(role.Permissions), required)
			if samePermissions(merged, role.Permissions) {
				continue
			}
			role.UpdatePermissions(merged)
			if err := h.roles.Save(ctx, role); err != nil {
				return err
			}
		}
	}
	return nil
}

func workspaceScopedPermissions(permissions []string) []string {
	filtered := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if strings.HasPrefix(strings.TrimSpace(permission), "org:") {
			continue
		}
		filtered = append(filtered, permission)
	}
	return filtered
}

func samePermissions(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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
		buttonIDs, err := h.menuStore.ListButtonIDsByRole(ctx, sourceID)
		if err != nil {
			return nil, err
		}
		if err := h.menuStore.SetButtonIDsForRole(ctx, copyID, buttonIDs); err != nil {
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
