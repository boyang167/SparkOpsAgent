package tenant

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/application/query"
	tenanthttp "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "tenant" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("tenant module requires database")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("tenant migrate: %w", err)
	}

	auditStore := audit.NewGormStore(c.DB)
	if err := auditStore.AutoMigrate(); err != nil {
		return fmt.Errorf("audit migrate: %w", err)
	}
	auditSvc := audit.NewService(auditStore)
	c.AuditLogger = auditSvc

	store := persistence.NewTenantStore(c.DB)
	orgRepo := persistence.NewOrganizationRepository(store)
	wsRepo := persistence.NewWorkspaceRepository(store)
	identityACL := acl.NewIdentityACL(c.DB)
	access := query.NewAccessChecker(store, orgRepo)
	c.WorkspaceMemberChecker = access
	c.OrgWorkspaceCreatorChecker = access
	c.OrgPermissionChecker = access

	handler := tenanthttp.NewHandler(
		command.NewCreateOrganizationHandler(orgRepo, auditSvc),
		command.NewCreateWorkspaceHandler(orgRepo, wsRepo, c.EventBus),
		query.NewListWorkspacesHandler(wsRepo, c.SystemRoleChecker),
		query.NewListOrganizationsHandler(orgRepo, c.SystemRoleChecker),
		query.NewGetOrganizationHandler(orgRepo),
		query.NewListOrgMembersHandler(orgRepo),
		query.NewMyOrgPermissionsHandler(orgRepo),
		query.NewSearchOrgMemberCandidatesHandler(identityACL),
		command.NewInviteOrgMemberHandler(orgRepo, identityACL, auditSvc),
		command.NewUpdateOrgMemberRoleHandler(orgRepo, auditSvc),
		command.NewRemoveOrgMemberHandler(orgRepo, auditSvc),
		query.NewListPlatformOrganizationsHandler(orgRepo),
		command.NewCreatePlatformOrganizationHandler(orgRepo, identityACL, auditSvc),
		command.NewUpdateOrganizationStatusHandler(orgRepo, auditSvc),
		command.NewDeletePlatformOrganizationHandler(orgRepo, auditSvc),
		command.NewTransferOrgOwnerHandler(orgRepo, identityACL, auditSvc),
		query.NewListAuditLogsHandler(auditStore),
		query.NewGetOrgQuotaHandler(orgRepo),
		query.NewGetOrgQuotaUsageHandler(orgRepo),
		command.NewUpdateOrgQuotaHandler(orgRepo),
		c.SystemRoleChecker,
		c.OrgPermissionChecker,
	)

	auth := authn.RequireAuth(m.tokens)
	superAdmin := authz.RequireSuperAdmin(c.SystemRoleChecker)
	org := httpx.RequireOrg()
	orgMember := authz.RequireOrgMember(c.OrgPermissionChecker)
	orgManage := authz.RequireOrgPermission(c.OrgPermissionChecker, authz.PermOrgMemberManage)
	orgRead := authz.RequireOrgPermission(c.OrgPermissionChecker, authz.PermOrgRead)
	orgWsCreate := authz.RequireOrgPermission(c.OrgPermissionChecker, authz.PermOrgWorkspaceCreate)

	c.Engine.GET("/api/v1/orgs", auth, handler.ListOrganizations)
	c.Engine.POST("/api/v1/orgs", auth, handler.CreateOrganization)
	c.Engine.GET("/api/v1/orgs/:orgId", auth, org, orgRead, handler.GetOrganization)
	c.Engine.GET("/api/v1/orgs/:orgId/me/permissions", auth, org, orgMember, handler.MyOrgPermissions)
	c.Engine.GET("/api/v1/orgs/:orgId/members", auth, org, orgManage, handler.ListOrgMembers)
	c.Engine.GET("/api/v1/orgs/:orgId/member-candidates", auth, org, orgManage, handler.SearchOrgMemberCandidates)
	c.Engine.POST("/api/v1/orgs/:orgId/members", auth, org, orgManage, handler.InviteOrgMember)
	c.Engine.PATCH("/api/v1/orgs/:orgId/members/:memberId", auth, org, orgManage, handler.UpdateOrgMemberRole)
	c.Engine.DELETE("/api/v1/orgs/:orgId/members/:memberId", auth, org, orgManage, handler.RemoveOrgMember)
	c.Engine.POST("/api/v1/orgs/:orgId/owners/transfer", auth, org, orgManage, handler.TransferOrgOwner)
	c.Engine.GET("/api/v1/orgs/:orgId/audit-logs", auth, org, orgManage, handler.ListOrgAuditLogs)
	c.Engine.GET("/api/v1/orgs/:orgId/quota", auth, org, orgRead, handler.GetOrgQuota)
	c.Engine.GET("/api/v1/orgs/:orgId/quota/usage", auth, org, orgRead, handler.GetOrgQuotaUsage)
	c.Engine.PATCH("/api/v1/orgs/:orgId/quota", auth, org, orgManage, handler.UpdateOrgQuota)
	c.Engine.PATCH("/api/v1/platform/orgs/:orgId/quota", auth, superAdmin, org, handler.UpdateOrgQuota)
	c.Engine.POST("/api/v1/orgs/:orgId/workspaces", auth, org, orgWsCreate, handler.CreateWorkspace)
	c.Engine.GET("/api/v1/workspaces", auth, handler.ListWorkspaces)

	c.Engine.GET("/api/v1/platform/orgs", auth, superAdmin, handler.ListPlatformOrganizations)
	c.Engine.POST("/api/v1/platform/orgs", auth, superAdmin, handler.CreatePlatformOrganization)
	c.Engine.PATCH("/api/v1/platform/orgs/:orgId/status", auth, superAdmin, org, handler.UpdatePlatformOrganizationStatus)
	c.Engine.DELETE("/api/v1/platform/orgs/:orgId", auth, superAdmin, org, handler.DeletePlatformOrganization)
	c.Engine.POST("/api/v1/platform/orgs/:orgId/owners/transfer", auth, superAdmin, org, handler.TransferPlatformOrgOwner)
	c.Engine.GET("/api/v1/platform/audit-logs", auth, superAdmin, handler.ListPlatformAuditLogs)

	return nil
}
