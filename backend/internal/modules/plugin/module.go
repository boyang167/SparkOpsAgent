package plugin

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/command"
	plugininstall "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence"
	pluginhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "plugin" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("plugin module requires database")
	}
	if c.Engine == nil {
		return fmt.Errorf("plugin module requires HTTP engine")
	}
	if c.SystemRoleChecker == nil {
		return fmt.Errorf("plugin module requires system role checker")
	}
	if m.tokens == nil {
		return fmt.Errorf("plugin module requires token provider")
	}
	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	publisher, err := plugininstall.NewGitCLIPublisherFromEnv(ctx, nil)
	if err != nil {
		return fmt.Errorf("plugin publisher: %w", err)
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("plugin migrate: %w", err)
	}

	repo := persistence.NewStore(c.DB)
	handler := pluginhttp.NewHandler(
		command.NewInstallHandler(repo, publisher),
		query.NewListHandler(repo),
		query.NewGetHandler(repo),
		command.NewMarkMergedHandler(repo),
		command.NewMarkDeployedHandler(repo),
		command.NewUninstallHandler(repo, publisher),
	)
	auth := authn.RequireAuth(m.tokens)
	superAdmin := authz.RequireSuperAdmin(c.SystemRoleChecker)
	c.Engine.POST("/api/v1/admin/plugins/install", auth, superAdmin, handler.Install)
	c.Engine.GET("/api/v1/admin/plugins", auth, superAdmin, handler.List)
	c.Engine.GET("/api/v1/admin/plugins/:id", auth, superAdmin, handler.Get)
	c.Engine.POST("/api/v1/admin/plugins/:id/merged", auth, superAdmin, handler.MarkMerged)
	c.Engine.POST("/api/v1/admin/plugins/:id/deployed", auth, superAdmin, handler.MarkDeployed)
	c.Engine.POST("/api/v1/admin/plugins/:id/uninstall", auth, superAdmin, handler.Uninstall)
	return nil
}
