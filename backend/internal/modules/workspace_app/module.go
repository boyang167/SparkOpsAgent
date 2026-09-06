package workspace_app

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	wshttp "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
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
	if err := command.SyncBuiltinTemplates(context.Background(), c.DB); err != nil {
		log.Printf("workspace_app: sync builtin templates: %v", err)
	}

	repo := persistence.NewProjectRepository(c.DB)
	memberRepo := persistence.NewProjectMemberRepository(c.DB)
	templateRepo := persistence.NewTemplateRepository(c.DB)
	secretRepo := persistence.NewSecretRepository(c.DB)
	wsChecker := acl.NewTenantACL(c.DB)
	clusterChecker := acl.NewWorkspaceClusterACL(c.DB)
	memberLookup := acl.NewMemberLookup(c.DB)
	secretBox, err := crypto.NewSecretBoxFromConfig(c.Config)
	if err != nil {
		return fmt.Errorf("secret crypto: %w", err)
	}
	secretCrypto := application.NewSecretCrypto(secretBox)

	if c.ProjectAccessChecker == nil {
		c.ProjectAccessChecker = authz.NewProjectAccessChecker(c.PermissionChecker, c.PolicyEngine, memberRepo)
	}
	if err := syncExistingProjectPolicies(c.DB, c.PolicyEngine); err != nil {
		return fmt.Errorf("sync project policies: %w", err)
	}

	generateTemplates := query.NewGenerateProjectTemplatesHandler(repo, templateRepo, secretRepo, wsChecker, c.DB)
	handler := wshttp.NewHandler(
		command.NewCreateProjectHandler(repo, memberRepo, wsChecker, clusterChecker, c.PolicyEngine, c.EventBus),
		command.NewUpdateProjectHandler(repo, wsChecker, clusterChecker),
		command.NewArchiveProjectHandler(repo, wsChecker),
		query.NewListProjectsHandler(repo, memberRepo, wsChecker, c.PermissionChecker),
		query.NewGetProjectHandler(repo, memberRepo, wsChecker, c.PermissionChecker),
		command.NewCreateTemplateHandler(templateRepo, wsChecker),
		command.NewUpdateTemplateHandler(templateRepo, wsChecker),
		command.NewDeleteTemplateHandler(templateRepo, wsChecker),
		query.NewListTemplatesHandler(templateRepo, wsChecker, memberLookup),
		query.NewGetTemplateHandler(templateRepo, wsChecker, memberLookup),
		command.NewCreateSecretHandler(secretRepo, wsChecker, secretCrypto),
		command.NewUpdateSecretHandler(secretRepo, wsChecker, secretCrypto, c.EventBus),
		command.NewDeleteSecretHandler(secretRepo, wsChecker),
		query.NewListSecretsHandler(secretRepo, wsChecker, memberLookup, secretCrypto),
		query.NewGetSecretHandler(secretRepo, wsChecker, memberLookup, secretCrypto),
		query.NewTestSecretConnectionHandler(secretRepo, wsChecker, secretCrypto),
		query.NewListGitRefsHandler(secretRepo, wsChecker, secretCrypto),
		query.NewListGitTreeHandler(secretRepo, wsChecker, secretCrypto),
		query.NewGetGitFileContentHandler(secretRepo, wsChecker, secretCrypto),
		secretCrypto,
		command.NewAddProjectMemberHandler(repo, memberRepo, wsChecker, c.PermissionChecker, c.PolicyEngine),
		command.NewUpdateProjectMemberRoleHandler(repo, memberRepo, wsChecker, c.PermissionChecker, c.PolicyEngine),
		command.NewRemoveProjectMemberHandler(repo, memberRepo, wsChecker, c.PermissionChecker, c.PolicyEngine),
		query.NewListProjectMembersHandler(repo, memberRepo),
		generateTemplates,
		command.NewUpdateGeneratedTemplateHandler(repo, wsChecker, generateTemplates),
		command.NewSubmitProjectReviewHandler(repo, c.ProjectAccessChecker, c.PermissionChecker),
		command.NewReviewProjectHandler(repo, c.ProjectAccessChecker, c.PermissionChecker),
		memberLookup,
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
	c.Engine.GET("/api/v1/projects/:projectId/generated-templates",
		auth, ws, wsMember, projectRead, handler.GenerateProjectTemplates)
	c.Engine.PUT("/api/v1/projects/:projectId/generated-templates/:kind",
		auth, ws, wsMember, projectWrite, handler.UpdateGeneratedTemplate)
	c.Engine.PATCH("/api/v1/projects/:projectId",
		auth, ws, wsMember, projectWrite, handler.UpdateProject)
	c.Engine.POST("/api/v1/projects/:projectId/submit-review",
		auth, ws, wsMember, projectWrite, handler.SubmitProjectReview)
	c.Engine.POST("/api/v1/projects/:projectId/review",
		auth, ws, wsMember, projectRead, handler.ReviewProject)
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
	c.Engine.GET("/api/v1/workspaces/:wsId/templates/:templateId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetTemplate)
	c.Engine.PATCH("/api/v1/workspaces/:wsId/templates/:templateId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateTemplate)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/templates/:templateId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteTemplate)
	c.Engine.GET("/api/v1/workspaces/:wsId/secrets",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermSecretRead), handler.ListSecrets)
	c.Engine.POST("/api/v1/workspaces/:wsId/secrets",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermSecretWrite), handler.CreateSecret)
	c.Engine.GET("/api/v1/workspaces/:wsId/secrets/:secretId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermSecretRead), handler.GetSecret)
	c.Engine.PATCH("/api/v1/workspaces/:wsId/secrets/:secretId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermSecretWrite), handler.UpdateSecret)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/secrets/:secretId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermSecretWrite), handler.DeleteSecret)
	c.Engine.POST("/api/v1/workspaces/:wsId/secrets/:secretId/test",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermSecretWrite), handler.TestSecretConnection)
	c.Engine.POST("/api/v1/workspaces/:wsId/git/refs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListGitRefs)
	c.Engine.POST("/api/v1/workspaces/:wsId/git/tree",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListGitTree)
	c.Engine.POST("/api/v1/workspaces/:wsId/git/file",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetGitFileContent)

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
