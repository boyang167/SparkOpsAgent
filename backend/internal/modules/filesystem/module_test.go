package filesystem

import (
	"context"
	"io"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	filesystemport "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type allowPermissions struct{}

func (allowPermissions) RequirePermission(context.Context, kernel.WorkspaceID, kernel.MemberID, string) error {
	return nil
}
func (allowPermissions) HasPermission(context.Context, kernel.WorkspaceID, kernel.MemberID, string) (bool, error) {
	return true, nil
}

type allowMembers struct{}

func (allowMembers) IsWorkspaceMember(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return true, nil
}

type discardAudit struct{}

func (discardAudit) Log(context.Context, audit.Entry) error { return nil }

func TestRegisterValidatesDependencies(t *testing.T) {
	registry := events.NewRegistry()
	base := appctx.Container{
		Config:                 &config.Config{},
		DB:                     &gorm.DB{},
		Transactor:             postgres.NewDirectTransactor(),
		EventBus:               messaging.NewInMemoryBus(registry),
		PermissionChecker:      allowPermissions{},
		WorkspaceMemberChecker: allowMembers{},
		AuditLogger:            discardAudit{},
		EventRegistry:          registry,
		Engine:                 httpx.NewEngine(),
	}
	tests := []struct {
		name string
		drop func(*appctx.Container)
		want string
	}{
		{name: "database", drop: func(c *appctx.Container) { c.DB = nil }, want: "requires database"},
		{name: "transactor", drop: func(c *appctx.Container) { c.Transactor = nil }, want: "requires transactor"},
		{name: "event bus", drop: func(c *appctx.Container) { c.EventBus = nil }, want: "requires event bus"},
		{name: "permission checker", drop: func(c *appctx.Container) { c.PermissionChecker = nil }, want: "requires permission checker"},
		{name: "member checker", drop: func(c *appctx.Container) { c.WorkspaceMemberChecker = nil }, want: "requires workspace member checker"},
		{name: "audit logger auto provision", drop: func(c *appctx.Container) {
			c.AuditLogger = nil
			db, openErr := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, openErr)
			require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
			require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
			c.DB = db
		}},
		{name: "event registry", drop: func(c *appctx.Container) { c.EventRegistry = nil }, want: "requires event registry"},
		{name: "engine", drop: func(c *appctx.Container) { c.Engine = nil }, want: "requires HTTP engine"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := base
			tt.drop(&container)
			err := New(authn.NewTokenProvider(config.JWTConfig{})).Register(&container)
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestRegisterExposesMetadataRoutesWithoutLocalProvider(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	registry := events.NewRegistry()
	engine := httpx.NewEngine()
	container := &appctx.Container{
		Config:                 &config.Config{},
		DB:                     db,
		Transactor:             postgres.NewTransactor(db),
		EventBus:               messaging.NewInMemoryBus(registry),
		PermissionChecker:      allowPermissions{},
		WorkspaceMemberChecker: allowMembers{},
		AuditLogger:            discardAudit{},
		EventRegistry:          registry,
		Engine:                 engine,
	}
	module := New(authn.NewTokenProvider(config.JWTConfig{}))
	require.NoError(t, module.Register(container))

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	require.True(t, routes["GET /api/v1/workspaces/:wsId/files"])
	require.True(t, routes["POST /api/v1/workspaces/:wsId/directories"])
	require.False(t, routes["POST /api/v1/workspaces/:wsId/files"], "upload is deferred to G2")
	require.NotNil(t, container.FilesystemGrantPort)
	repo := persistence.NewRepository(db)
	owner := uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	root, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), root))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), namespace.WorkspaceID, root.ID))
	grants, err := container.FilesystemGrantPort.List(context.Background(), filesystemport.GrantRequest{
		ActorID: owner, OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID,
		GranteeOrganizationID: namespace.OrganizationID, NamespaceID: namespace.ID, NodeID: root.ID,
	})
	require.NoError(t, err)
	require.Empty(t, grants)

	err = module.Register(container)
	require.ErrorContains(t, err, "duplicate event descriptor")
	closer, ok := any(module).(io.Closer)
	require.True(t, ok, "filesystem module must release its runtime catalog")
	require.NoError(t, closer.Close())
}

var _ authz.PermissionChecker = allowPermissions{}
var _ authz.WorkspaceMemberChecker = allowMembers{}
var _ audit.Logger = discardAudit{}
