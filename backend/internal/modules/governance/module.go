package governance

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/subscriber"
	govhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
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

	checker := query.NewChecker(bindingRepo)
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
	apiPolicySync := service.NewAPIPolicySync(engine, roleRepo, bindingRepo, roleAPIPolicyStore)
	if err := apiPolicySync.Bootstrap(context.Background()); err != nil {
		return fmt.Errorf("api policy casbin bootstrap: %w", err)
	}
	c.APIPolicyChecker = apiPolicySync

	createRole := command.NewCreateRoleHandler(roleRepo)
	updateRole := command.NewUpdateRoleHandler(roleRepo)
	deleteRole := command.NewDeleteRoleHandler(roleRepo, bindingRepo, roleAPIPolicyStore, apiPolicySync)
	copyRole := command.NewCopyRoleHandler(roleRepo)
	assignRole := command.NewAssignRoleHandler(roleRepo, bindingRepo, apiPolicySync)
	inviteMember := command.NewInviteWorkspaceMemberHandler(acl.NewIdentityACL(c.DB), acl.NewTenantACL(c.DB))
	removeBinding := command.NewRemoveRoleBindingHandler(roleRepo, bindingRepo, apiPolicySync)
	listRoles := query.NewListRolesHandler(roleRepo)
	listRoleTree := query.NewListRoleTreeHandler(roleRepo)
	listMembers := query.NewListWorkspaceMembersHandler(acl.NewMemberACL(c.DB), bindingRepo)
	listMemberRoles := query.NewListMemberRolesHandler(bindingRepo)
	bootstrap := command.NewBootstrapWorkspaceHandler(roleRepo, bindingRepo, apiPolicySync)

	c.EventBus.Subscribe(integration.EventWorkspaceCreated, subscriber.NewOnWorkspaceCreated(bootstrap).Handle)

	handler := govhttp.NewHandler(
		createRole, updateRole, deleteRole, copyRole, assignRole, inviteMember, removeBinding,
		listRoles, listRoleTree, listMembers, listMemberRoles, checker,
	)

	platformAPI := govhttp.NewPlatformAPIHandler(apiRegistry, c.Engine)
	createAPIToken := identitycommand.NewCreateAPITokenHandler(m.tokens, apiTokenStore, roleRepo)
	revokeAPIToken := identitycommand.NewRevokeAPITokenHandler(apiTokenStore, m.tokens)
	apiTokenHandler := govhttp.NewAPITokenHandler(createAPIToken, revokeAPIToken, apiTokenStore)
	roleAPIPolicyHandler := govhttp.NewRoleAPIPolicyHandler(
		query.NewListRoleAPIPoliciesHandler(roleRepo, roleAPIPolicyStore),
		command.NewUpdateRoleAPIPoliciesHandler(roleRepo, roleAPIPolicyStore, apiPolicySync),
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
	c.Engine.DELETE("/api/v1/workspaces/:wsId/roles/:roleId",
		auth, ws, wsMember, govAdmin, handler.DeleteRole)
	c.Engine.GET("/api/v1/workspaces/:wsId/roles/:roleId/api-policies",
		auth, ws, wsMember, govAdmin, roleAPIPolicyHandler.List)
	c.Engine.PUT("/api/v1/workspaces/:wsId/roles/:roleId/api-policies",
		auth, ws, wsMember, govAdmin, roleAPIPolicyHandler.Update)
	c.Engine.POST("/api/v1/workspaces/:wsId/members/:memberId/roles",
		auth, ws, wsMember, govAdmin, handler.AssignRole)
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
	platform.PATCH("/apis/:apiId", auth, superAdmin, platformAPI.UpdateMeta)

	return nil
}
