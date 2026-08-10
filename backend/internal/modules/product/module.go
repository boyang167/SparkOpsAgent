package product

import (
	"fmt"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	producthttp "github.com/sparkops/sparkops-agent/backend/internal/modules/product/interface/http"
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

func (m *Module) Name() string { return "product" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("product module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("product module requires permission checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("product migrate: %w", err)
	}

	repo := persistence.NewProductRepository(c.DB)
	wsChecker := acl.NewTenantACL(c.DB)

	handler := producthttp.NewHandler(
		command.NewCreateProductHandler(repo, wsChecker),
		command.NewUpdateProductHandler(repo, wsChecker),
		command.NewCreateProductPlanHandler(repo),
		command.NewCreateProductRequirementHandler(repo),
		command.NewApproveRequirementHandler(repo),
		command.NewRejectRequirementHandler(repo),
		command.NewReleaseRequirementHandler(repo, c.BuildTrigger),
		command.NewCrossReleaseRequirementHandler(repo, c.SyncJobCreator),
		query.NewListProductsHandler(repo, wsChecker),
		query.NewGetProductHandler(repo, wsChecker),
		query.NewListProductPlansHandler(repo),
		query.NewListProductRequirementsHandler(repo),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.GET("/api/v1/workspaces/:wsId/products",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListProducts)
	c.Engine.POST("/api/v1/workspaces/:wsId/products",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateProduct)
	c.Engine.GET("/api/v1/products/:productId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetProduct)
	c.Engine.PATCH("/api/v1/products/:productId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateProduct)
	c.Engine.GET("/api/v1/products/:productId/plans",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListPlans)
	c.Engine.POST("/api/v1/products/:productId/plans",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreatePlan)
	c.Engine.GET("/api/v1/products/:productId/requirements",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListRequirements)
	c.Engine.POST("/api/v1/products/:productId/requirements",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateRequirement)
	c.Engine.POST("/api/v1/products/:productId/requirements/:reqId/approve",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ApproveRequirement)
	c.Engine.POST("/api/v1/products/:productId/requirements/:reqId/reject",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.RejectRequirement)
	c.Engine.POST("/api/v1/products/:productId/requirements/:reqId/release",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ReleaseRequirement)
	c.Engine.POST("/api/v1/products/:productId/requirements/:reqId/cross-release",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CrossReleaseRequirement)

	c.EventBus.Subscribe(deliveryevent.BuildCompletedName, subscriber.NewOnBuildCompleted(repo).Handle)

	return nil
}
