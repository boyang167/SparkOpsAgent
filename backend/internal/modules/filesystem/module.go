package filesystem

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/subscriber"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	filesystemhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "filesystem" }

func (m *Module) Register(c *appctx.Container) error {
	if c == nil {
		return fmt.Errorf("filesystem module requires container")
	}
	if c.DB == nil {
		return fmt.Errorf("filesystem module requires database")
	}
	if c.Transactor == nil {
		return fmt.Errorf("filesystem module requires transactor")
	}
	if c.EventBus == nil {
		return fmt.Errorf("filesystem module requires event bus")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("filesystem module requires permission checker")
	}
	if c.WorkspaceMemberChecker == nil {
		return fmt.Errorf("filesystem module requires workspace member checker")
	}
	if c.AuditLogger == nil {
		return fmt.Errorf("filesystem module requires audit logger")
	}
	if c.EventRegistry == nil {
		return fmt.Errorf("filesystem module requires event registry")
	}
	if c.Engine == nil {
		return fmt.Errorf("filesystem module requires HTTP engine")
	}
	if err := filesystemevent.Register(c.EventRegistry); err != nil {
		return fmt.Errorf("filesystem event codecs: %w", err)
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("filesystem migrate: %w", err)
	}

	repo := persistence.NewRepository(c.DB)
	createNamespace := command.NewCreateNamespaceHandler(repo, c.Transactor, c.EventBus, c.AuditLogger)
	if err := c.EventBus.Subscribe(
		integration.EventWorkspaceCreated,
		"filesystem.workspace-created.create-namespace",
		subscriber.NewOnWorkspaceCreated(createNamespace).Handle,
	); err != nil {
		return fmt.Errorf("subscribe filesystem workspace bootstrap: %w", err)
	}
	handler := filesystemhttp.NewHandler(
		repo,
		query.NewListChildrenHandler(repo),
		query.NewListRevisionsHandler(repo),
		command.NewCreateDirectoryHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewRenameNodeHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewMoveNodeHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewCopyNodeHandler(repo, c.Transactor, c.AuditLogger),
		command.NewTrashNodeHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
		command.NewRestoreRevisionHandler(repo, c.Transactor, c.EventBus, c.AuditLogger),
	)
	filesystemhttp.RegisterRoutes(
		c.Engine,
		entitlements.Middleware(c.DB),
		authn.RequireAuth(m.tokens),
		c.WorkspaceMemberChecker,
		c.PermissionChecker,
		handler,
	)
	return nil
}
