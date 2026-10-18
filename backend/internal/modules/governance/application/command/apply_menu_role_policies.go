package command

import (
	"context"
	"errors"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type ApplyMenuRolePoliciesOptions struct {
	// SkipIfConfigured keeps existing role_menus bindings on a menu (default true).
	SkipIfConfigured bool
}

type ApplyMenuRolePoliciesHandler struct {
	menus     *persistence.MenuStore
	roles     repository.RoleRepository
	seedMenus *SeedWorkspaceMenusHandler
}

func NewApplyMenuRolePoliciesHandler(
	menus *persistence.MenuStore,
	roles repository.RoleRepository,
	seedMenus *SeedWorkspaceMenusHandler,
) *ApplyMenuRolePoliciesHandler {
	return &ApplyMenuRolePoliciesHandler{menus: menus, roles: roles, seedMenus: seedMenus}
}

func (h *ApplyMenuRolePoliciesHandler) EnsureWorkspaceMenus(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
) error {
	return h.Handle(ctx, workspaceID, ApplyMenuRolePoliciesOptions{SkipIfConfigured: true})
}

func (h *ApplyMenuRolePoliciesHandler) HandleAllWorkspaces(ctx context.Context, opts ApplyMenuRolePoliciesOptions) error {
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
		if err := h.Handle(ctx, workspaceID, opts); err != nil {
			return err
		}
	}
	return nil
}

func (h *ApplyMenuRolePoliciesHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	opts ApplyMenuRolePoliciesOptions,
) error {
	if h.seedMenus != nil {
		if err := h.seedMenus.Handle(ctx, workspaceID); err != nil {
			return err
		}
	}
	roles, err := h.roles.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	roleIDsByName := make(map[string]string, len(roles))
	for _, role := range roles {
		roleIDsByName[role.Name] = role.ID.String()
	}

	for path, roleNames := range DefaultMenuRolePolicies {
		menu, err := h.menus.FindByPath(ctx, workspaceID, path)
		if err != nil {
			if errors.Is(err, domain.ErrMenuNotFound) {
				continue
			}
			return err
		}
		if opts.SkipIfConfigured {
			existing, err := h.menus.ListRoleIDs(ctx, menu.ID)
			if err != nil {
				return err
			}
			if len(existing) > 0 {
				continue
			}
		}
		roleIDs := resolveMenuPolicyRoleIDs(roleNames, roleIDsByName)
		if len(roleIDs) == 0 {
			continue
		}
		if err := h.menus.SetRoleIDs(ctx, menu.ID, roleIDs); err != nil {
			return err
		}
	}
	return nil
}

func resolveMenuPolicyRoleIDs(roleNames []string, roleIDsByName map[string]string) []string {
	seen := make(map[string]struct{}, len(roleNames))
	out := make([]string, 0, len(roleNames))
	for _, name := range roleNames {
		id, ok := roleIDsByName[name]
		if !ok || id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// EnsureMenuRolePolicies is a convenience wrapper used at startup and seed time.
func EnsureMenuRolePolicies(
	ctx context.Context,
	db *gorm.DB,
	opts ApplyMenuRolePoliciesOptions,
) error {
	store := persistence.NewStore(db)
	menuStore := persistence.NewMenuStore(db)
	roleRepo := persistence.NewRoleRepository(store)
	handler := NewApplyMenuRolePoliciesHandler(
		menuStore,
		roleRepo,
		NewSeedWorkspaceMenusHandler(menuStore),
	)
	return handler.HandleAllWorkspaces(ctx, opts)
}
