package workspace_app

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	wshttp "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "workspace_app" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("workspace_app module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("workspace_app module requires permission checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("workspace_app migrate: %w", err)
	}

	repo := persistence.NewProjectRepository(c.DB)
	memberRepo := persistence.NewProjectMemberRepository(c.DB)
	templateRepo := persistence.NewTemplateRepository(c.DB)
	marketRepo := persistence.NewMarketRepository(c.DB)
	wsChecker := acl.NewTenantACL(c.DB)

	if c.ProjectAccessChecker == nil {
		c.ProjectAccessChecker = authz.NewProjectAccessChecker(c.PermissionChecker, c.PolicyEngine, memberRepo)
	}
	if err := syncExistingProjectPolicies(c.DB, c.PolicyEngine); err != nil {
		return fmt.Errorf("sync project policies: %w", err)
	}

	handler := wshttp.NewHandler(
		command.NewCreateProjectHandler(repo, memberRepo, wsChecker, c.PolicyEngine, c.EventBus),
		command.NewUpdateProjectHandler(repo, wsChecker),
		command.NewArchiveProjectHandler(repo, wsChecker),
		query.NewListProjectsHandler(repo, memberRepo, wsChecker, c.PermissionChecker),
		query.NewGetProjectHandler(repo, wsChecker),
		command.NewCreateTemplateHandler(templateRepo, wsChecker),
		query.NewListTemplatesHandler(templateRepo, wsChecker),
		command.NewCreateDockerfileHandler(marketRepo, wsChecker),
		query.NewListDockerfilesHandler(marketRepo, wsChecker),
		command.NewAddProjectMemberHandler(repo, memberRepo, wsChecker, c.PermissionChecker, c.PolicyEngine),
		command.NewUpdateProjectMemberRoleHandler(repo, memberRepo, wsChecker, c.PermissionChecker, c.PolicyEngine),
		command.NewRemoveProjectMemberHandler(repo, memberRepo, wsChecker, c.PermissionChecker, c.PolicyEngine),
		query.NewListProjectMembersHandler(repo, memberRepo),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)
	projectRead := authz.RequireProjectAccess(c.ProjectAccessChecker, policy.ActionRead)
	projectWrite := authz.RequireProjectAccess(c.ProjectAccessChecker, policy.ActionWrite)
	projectAdmin := authz.RequireProjectAccess(c.ProjectAccessChecker, policy.ActionAdmin)

	c.Engine.POST("/api/v1/workspaces/:wsId/projects",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateProject)
	c.Engine.GET("/api/v1/workspaces/:wsId/projects",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListProjects)
	c.Engine.GET("/api/v1/projects/:projectId",
		auth, ws, wsMember, projectRead, handler.GetProject)
	c.Engine.PATCH("/api/v1/projects/:projectId",
		auth, ws, wsMember, projectWrite, handler.UpdateProject)
	c.Engine.DELETE("/api/v1/projects/:projectId",
		auth, ws, wsMember, projectAdmin, handler.ArchiveProject)
	c.Engine.GET("/api/v1/projects/:projectId/members",
		auth, ws, wsMember, projectRead, handler.ListProjectMembers)
	c.Engine.POST("/api/v1/projects/:projectId/members",
		auth, ws, wsMember, projectAdmin, handler.AddProjectMember)
	c.Engine.PATCH("/api/v1/projects/:projectId/members/:memberId",
		auth, ws, wsMember, projectAdmin, handler.UpdateProjectMemberRole)
	c.Engine.DELETE("/api/v1/projects/:projectId/members/:memberId",
		auth, ws, wsMember, projectAdmin, handler.RemoveProjectMember)
	c.Engine.GET("/api/v1/workspaces/:wsId/templates",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListTemplates)
	c.Engine.POST("/api/v1/workspaces/:wsId/templates",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateTemplate)
	c.Engine.GET("/api/v1/workspaces/:wsId/market/dockerfiles",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListDockerfiles)
	c.Engine.POST("/api/v1/workspaces/:wsId/market/dockerfiles",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateDockerfile)

	return nil
}

func syncExistingProjectPolicies(db *gorm.DB, engine policy.Engine) error {
	ce, ok := engine.(*policy.CasbinEngine)
	if !ok || ce == nil {
		return nil
	}
	type row struct {
		ProjectID   string
		WorkspaceID string
		MemberID    string
		Role        string
	}
	var rows []row
	err := db.Raw(`
		SELECT pm.project_id, p.workspace_id, pm.member_id, pm.role
		FROM workspace_app.project_members pm
		JOIN workspace_app.projects p ON p.id = pm.project_id
		WHERE pm.deleted_at IS NULL
	`).Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, r := range rows {
		_ = ce.SyncProjectMemberRole(r.MemberID, r.WorkspaceID, r.ProjectID, r.Role)
	}
	return nil
}
