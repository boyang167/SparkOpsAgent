package governance

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	govhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/interface/http"
	identitycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	identitypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "governance" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("governance module requires database")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("governance migrate: %w", err)
	}
	apiRegistry := persistence.NewAPIRegistryStore(c.DB)
	if err := apiRegistry.AutoMigrate(); err != nil {
		return fmt.Errorf("api registry migrate: %w", err)
	}
	apiTokenStore := identitypersistence.NewAPITokenStore(c.DB)
	if err := apiTokenStore.AutoMigrate(); err != nil {
		return fmt.Errorf("api token migrate: %w", err)
	}

	store := persistence.NewStore(c.DB)
	roleRepo := persistence.NewRoleRepository(store)
	bindingRepo := persistence.NewBindingRepository(store)

	checker := query.NewChecker(bindingRepo, c.SystemRoleChecker)
	c.PermissionChecker = checker

	engine, err := policy.NewEngine(c.Config.Policy, c.DB)
	if err != nil {
		return fmt.Errorf("policy engine: %w", err)
	}
	c.PolicyEngine = engine

	roleAPIPolicyStore := persistence.NewRoleAPIPolicyStore(c.DB)
	if err := roleAPIPolicyStore.AutoMigrate(); err != nil {
		return fmt.Errorf("role api policy migrate: %w", err)
	}
	dataPolicyStore := persistence.NewDataPolicyStore(c.DB)
	if err := dataPolicyStore.AutoMigrate(); err != nil {
		return fmt.Errorf("data policy migrate: %w", err)
	}
	menuStore := persistence.NewMenuStore(c.DB)
	if err := menuStore.AutoMigrate(); err != nil {
		return fmt.Errorf("menu migrate: %w", err)
	}
	apiPolicySync := service.NewAPIPolicySync(engine, roleRepo, bindingRepo, roleAPIPolicyStore)
	if err := apiPolicySync.Bootstrap(context.Background()); err != nil {
		return fmt.Errorf("api policy casbin bootstrap: %w", err)
	}
	c.APIPolicyChecker = apiPolicySync

	createRole := command.NewCreateRoleHandler(roleRepo)
	updateRole := command.NewUpdateRoleHandler(roleRepo)
	deleteRole := command.NewDeleteRoleHandler(roleRepo, bindingRepo, roleAPIPolicyStore, dataPolicyStore, apiPolicySync)
	copyRole := command.NewCopyRoleHandler(roleRepo, roleAPIPolicyStore, menuStore, dataPolicyStore, apiPolicySync)
	assignRole := command.NewAssignRoleHandler(roleRepo, bindingRepo, apiPolicySync)
	identityACL := acl.NewIdentityACL(c.DB)
	inviteMember := command.NewInviteWorkspaceMemberHandler(identityACL, acl.NewTenantACL(c.DB))
	removeBinding := command.NewRemoveRoleBindingHandler(roleRepo, bindingRepo, apiPolicySync)
	listRoles := query.NewListRolesHandler(roleRepo)
	listRoleTree := query.NewListRoleTreeHandler(roleRepo)
	listMembers := query.NewListWorkspaceMembersHandler(acl.NewMemberACL(c.DB), bindingRepo)
	listMemberRoles := query.NewListMemberRolesHandler(bindingRepo)
	searchCandidates := query.NewSearchMemberCandidatesHandler(identityACL)
	bootstrap := command.NewBootstrapWorkspaceHandler(roleRepo, bindingRepo, apiPolicySync)
	ensureDefaultRoles := command.NewEnsureDefaultWorkspaceRolesHandler(roleRepo)
	if err := ensureDefaultRoles.Handle(context.Background()); err != nil {
		return fmt.Errorf("ensure default workspace roles: %w", err)
	}

	seedMenus := command.NewSeedWorkspaceMenusHandler(menuStore)
	if err := seedMenus.RepairMisdeletedPluginMenusAllWorkspaces(context.Background()); err != nil {
		return fmt.Errorf("repair misdeleted plugin menus: %w", err)
	}
	if err := seedMenus.RepairRemovedMenusAllWorkspaces(context.Background()); err != nil {
		return fmt.Errorf("repair removed menus: %w", err)
	}
	menuPolicies := command.NewApplyMenuRolePoliciesHandler(menuStore, roleRepo, seedMenus)
	if err := menuPolicies.HandleAllWorkspaces(context.Background(), command.ApplyMenuRolePoliciesOptions{SkipIfConfigured: true}); err != nil {
		return fmt.Errorf("apply menu role policies: %w", err)
	}
	if err := c.EventBus.Subscribe(
		integration.EventWorkspaceCreated,
		"governance.workspace-created.bootstrap",
		subscriber.NewOnWorkspaceCreated(bootstrap, menuPolicies).Handle,
	); err != nil {
		return fmt.Errorf("subscribe governance workspace bootstrap: %w", err)
	}

	handler := govhttp.NewHandler(
		createRole, updateRole, deleteRole, copyRole, assignRole, inviteMember, removeBinding,
		listRoles, listRoleTree, listMembers, listMemberRoles, searchCandidates, checker,
	)

	platformAPI := govhttp.NewPlatformAPIHandler(apiRegistry, c.Engine)
	createAPIToken := identitycommand.NewCreateAPITokenHandler(m.tokens, apiTokenStore, roleRepo)
	revokeAPIToken := identitycommand.NewRevokeAPITokenHandler(apiTokenStore, m.tokens)
	apiTokenHandler := govhttp.NewAPITokenHandler(createAPIToken, revokeAPIToken, apiTokenStore)
	roleAPIPolicyHandler := govhttp.NewRoleAPIPolicyHandler(
		query.NewListRoleAPIPoliciesHandler(roleRepo, roleAPIPolicyStore),
		command.NewUpdateRoleAPIPoliciesHandler(roleRepo, roleAPIPolicyStore, apiPolicySync),
	)
	dataPolicyHandler := govhttp.NewDataPolicyHandler(
		query.NewListDataResourcesHandler(dataPolicyStore),
		query.NewListRoleDataPoliciesHandler(roleRepo, dataPolicyStore),
		command.NewUpdateRoleDataPoliciesHandler(roleRepo, dataPolicyStore),
	)

	menuHandler := govhttp.NewMenuHandler(
		query.NewListMenuTreeHandler(menuStore, seedMenus),
		query.NewListNavMenusHandler(menuStore, seedMenus, checker, c.SystemRoleChecker, c.DB),
		query.NewListMyMenuButtonsHandler(menuStore, checker, c.SystemRoleChecker),
		query.NewGetMenuHandler(menuStore),
		query.NewGetMenuRolesHandler(menuStore),
		query.NewGetRoleMenusHandler(menuStore, roleRepo),
		query.NewListWorkspaceMenuButtonsHandler(menuStore, seedMenus),
		query.NewGetRoleMenuButtonsHandler(menuStore, roleRepo),
		command.NewCreateMenuHandler(menuStore),
		command.NewUpdateMenuHandler(menuStore),
		command.NewDeleteMenuHandler(menuStore),
		command.NewSetMenuRolesHandler(menuStore, roleRepo),
		command.NewSetRoleMenusHandler(menuStore, roleRepo),
		command.NewSetRoleMenuButtonsHandler(menuStore, roleRepo),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)
	govAdmin := authz.RequirePermission(checker, authz.PermGovernanceAdmin)
	superAdmin := authz.RequireSuperAdmin(c.SystemRoleChecker)

	c.Engine.GET("/api/v1/workspaces/:wsId/roles/tree",
		auth, ws, wsMember, govAdmin, handler.ListRoleTree)
	c.Engine.GET("/api/v1/workspaces/:wsId/roles",
		auth, ws, wsMember, govAdmin, handler.ListRoles)
	c.Engine.POST("/api/v1/workspaces/:wsId/roles",
		auth, ws, wsMember, govAdmin, handler.CreateRole)
	c.Engine.POST("/api/v1/workspaces/:wsId/roles/:roleId/copy",
		auth, ws, wsMember, govAdmin, handler.CopyRole)
	c.Engine.PATCH("/api/v1/workspaces/:wsId/roles/:roleId",
		auth, ws, wsMember, govAdmin, handler.UpdateRole)
	c.Engine.GET("/api/v1/workspaces/:wsId/roles/:roleId/menus",
		auth, ws, wsMember, govAdmin, menuHandler.GetRoleMenus)
	c.Engine.PUT("/api/v1/workspaces/:wsId/roles/:roleId/menus",
		auth, ws, wsMember, govAdmin, menuHandler.SetRoleMenus)
	c.Engine.GET("/api/v1/workspaces/:wsId/menu-buttons",
		auth, ws, wsMember, govAdmin, menuHandler.ListWorkspaceMenuButtons)
	c.Engine.GET("/api/v1/workspaces/:wsId/roles/:roleId/menu-buttons",
		auth, ws, wsMember, govAdmin, menuHandler.GetRoleMenuButtons)
	c.Engine.PUT("/api/v1/workspaces/:wsId/roles/:roleId/menu-buttons",
		auth, ws, wsMember, govAdmin, menuHandler.SetRoleMenuButtons)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/roles/:roleId",
		auth, ws, wsMember, govAdmin, handler.DeleteRole)
	c.Engine.GET("/api/v1/workspaces/:wsId/roles/:roleId/api-policies",
		auth, ws, wsMember, govAdmin, roleAPIPolicyHandler.List)
	c.Engine.PUT("/api/v1/workspaces/:wsId/roles/:roleId/api-policies",
		auth, ws, wsMember, govAdmin, roleAPIPolicyHandler.Update)
	c.Engine.GET("/api/v1/workspaces/:wsId/data-resources",
		auth, ws, wsMember, govAdmin, dataPolicyHandler.ListResources)
	c.Engine.GET("/api/v1/workspaces/:wsId/roles/:roleId/data-policies",
		auth, ws, wsMember, govAdmin, dataPolicyHandler.ListRolePolicies)
	c.Engine.PUT("/api/v1/workspaces/:wsId/roles/:roleId/data-policies",
		auth, ws, wsMember, govAdmin, dataPolicyHandler.UpdateRolePolicies)
	c.Engine.POST("/api/v1/workspaces/:wsId/members/:memberId/roles",
		auth, ws, wsMember, govAdmin, handler.AssignRole)
	c.Engine.GET("/api/v1/workspaces/:wsId/member-candidates",
		auth, ws, wsMember, govAdmin, handler.SearchMemberCandidates)
	c.Engine.GET("/api/v1/workspaces/:wsId/members",
		auth, ws, wsMember, govAdmin, handler.ListMembers)
	c.Engine.POST("/api/v1/workspaces/:wsId/members",
		auth, ws, wsMember, govAdmin, handler.InviteMember)
	c.Engine.GET("/api/v1/workspaces/:wsId/members/:memberId/roles",
		auth, ws, wsMember, govAdmin, handler.ListMemberRoles)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/members/:memberId/roles/:roleId",
		auth, ws, wsMember, govAdmin, handler.RemoveRoleBinding)
	c.Engine.GET("/api/v1/workspaces/:wsId/me/permissions",
		auth, ws, wsMember, handler.MyPermissions)
	c.Engine.GET("/api/v1/workspaces/:wsId/apis",
		auth, ws, wsMember, govAdmin, platformAPI.List)
	c.Engine.GET("/api/v1/workspaces/:wsId/apis/sync",
		auth, ws, wsMember, govAdmin, platformAPI.SyncPreview)
	c.Engine.POST("/api/v1/workspaces/:wsId/apis/sync",
		auth, ws, wsMember, govAdmin, platformAPI.SyncConfirm)
	c.Engine.POST("/api/v1/workspaces/:wsId/apis/batch-delete",
		auth, ws, wsMember, govAdmin, platformAPI.BatchDelete)
	c.Engine.PATCH("/api/v1/workspaces/:wsId/apis/:apiId",
		auth, ws, wsMember, govAdmin, platformAPI.UpdateMeta)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/apis/:apiId",
		auth, ws, wsMember, govAdmin, platformAPI.Delete)

	c.Engine.GET("/api/v1/workspaces/:wsId/me/menus",
		auth, ws, wsMember, menuHandler.ListNav)
	c.Engine.GET("/api/v1/workspaces/:wsId/me/menu-buttons",
		auth, ws, wsMember, menuHandler.ListMyButtons)
	c.Engine.GET("/api/v1/workspaces/:wsId/menus/tree",
		auth, ws, wsMember, govAdmin, menuHandler.ListTree)
	c.Engine.POST("/api/v1/workspaces/:wsId/menus",
		auth, ws, wsMember, govAdmin, menuHandler.Create)
	c.Engine.GET("/api/v1/workspaces/:wsId/menus/:menuId",
		auth, ws, wsMember, govAdmin, menuHandler.Get)
	c.Engine.PATCH("/api/v1/workspaces/:wsId/menus/:menuId",
		auth, ws, wsMember, govAdmin, menuHandler.Update)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/menus/:menuId",
		auth, ws, wsMember, govAdmin, menuHandler.Delete)
	c.Engine.GET("/api/v1/workspaces/:wsId/menus/:menuId/roles",
		auth, ws, wsMember, govAdmin, menuHandler.GetRoles)
	c.Engine.PUT("/api/v1/workspaces/:wsId/menus/:menuId/roles",
		auth, ws, wsMember, govAdmin, menuHandler.SetRoles)

	c.Engine.GET("/api/v1/workspaces/:wsId/api-tokens",
		auth, ws, wsMember, govAdmin, apiTokenHandler.List)
	c.Engine.POST("/api/v1/workspaces/:wsId/api-tokens",
		auth, ws, wsMember, govAdmin, apiTokenHandler.Create)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/api-tokens/:tokenId",
		auth, ws, wsMember, govAdmin, apiTokenHandler.Revoke)

	platform := c.Engine.Group("/api/v1/platform")
	platform.GET("/apis", auth, superAdmin, platformAPI.List)
	platform.GET("/apis/sync", auth, superAdmin, platformAPI.SyncPreview)
	platform.POST("/apis/sync", auth, superAdmin, platformAPI.SyncConfirm)
	platform.POST("/apis/batch-delete", auth, superAdmin, platformAPI.BatchDelete)
	platform.PATCH("/apis/:apiId", auth, superAdmin, platformAPI.UpdateMeta)
	platform.DELETE("/apis/:apiId", auth, superAdmin, platformAPI.Delete)

	return nil
}
