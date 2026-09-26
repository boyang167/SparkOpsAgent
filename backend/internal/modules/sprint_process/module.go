package sprint_process

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/infrastructure/persistence"
	sphttp "github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
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

func (m *Module) Name() string { return "sprint_process" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("sprint_process module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("sprint_process module requires permission checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("sprint_process migrate: %w", err)
	}

	productChecker := acl.NewProductChecker(c.DB)
	sprintUsage := acl.NewSprintUsageChecker(c.DB)
	processRepo := persistence.NewProcessRepository(c.DB)
	templateRepo := persistence.NewWorkspaceTemplateRepository(c.DB)

	getOrSeed := command.NewGetOrSeedSprintProcessHandler(productChecker, processRepo)
	upsertProcess := command.NewUpsertSprintProcessHandler(processRepo, getOrSeed)
	listProcesses := command.NewListSprintProcessesHandler(productChecker, processRepo, getOrSeed)
	createProcess := command.NewCreateSprintProcessHandler(productChecker, processRepo)
	updateProcess := command.NewUpdateSprintProcessByIDHandler(productChecker, processRepo)
	deleteProcess := command.NewDeleteSprintProcessHandler(productChecker, processRepo, sprintUsage)
	importWs := command.NewImportWorkspaceTemplateToProductHandler(productChecker, processRepo, templateRepo)
	listWsTpl := command.NewListWorkspaceSprintTemplatesHandler(templateRepo)
	createWsTpl := command.NewCreateWorkspaceSprintTemplateHandler(templateRepo)
	updateWsTpl := command.NewUpdateWorkspaceSprintTemplateHandler(templateRepo)
	deleteWsTpl := command.NewDeleteWorkspaceSprintTemplateHandler(templateRepo)

	handler := sphttp.NewHandler(
		getOrSeed, upsertProcess, listProcesses, createProcess, updateProcess, deleteProcess,
		importWs, listWsTpl, createWsTpl, updateWsTpl, deleteWsTpl,
	)

	c.SprintProcessPort = application.NewSprintProcessPort(getOrSeed, processRepo)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.GET("/api/v1/workspaces/:wsId/sprint-process-templates",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListWorkspaceSprintTemplates)
	c.Engine.POST("/api/v1/workspaces/:wsId/sprint-process-templates",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateWorkspaceSprintTemplate)
	c.Engine.PUT("/api/v1/workspaces/:wsId/sprint-process-templates/:templateId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateWorkspaceSprintTemplate)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/sprint-process-templates/:templateId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteWorkspaceSprintTemplate)
	c.Engine.GET("/api/v1/products/:productId/sprint-process",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetSprintProcess)
	c.Engine.PUT("/api/v1/products/:productId/sprint-process",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpsertSprintProcess)
	c.Engine.GET("/api/v1/products/:productId/sprint-processes",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListSprintProcesses)
	c.Engine.POST("/api/v1/products/:productId/sprint-processes",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateSprintProcess)
	c.Engine.PUT("/api/v1/products/:productId/sprint-processes/:processId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateSprintProcessByID)
	c.Engine.DELETE("/api/v1/products/:productId/sprint-processes/:processId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteSprintProcess)
	c.Engine.POST("/api/v1/workspaces/:wsId/products/:productId/sprint-processes/import-workspace",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ImportWorkspaceSprintTemplate)

	return nil
}
