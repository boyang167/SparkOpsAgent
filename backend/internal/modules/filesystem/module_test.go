package filesystem

import (
	"context"
	"io"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	filesystemquery "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	filesystemport "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	govquery "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/query"
	govmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
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

type allowMembers struct{ organizationID uuid.UUID }

func (*allowMembers) IsWorkspaceMember(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return true, nil
}
func (a *allowMembers) ResolveWorkspaceAccess(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID) (kernel.OrgID, bool, error) {
	return kernel.OrgID{UUID: a.organizationID}, true, nil
}
func (a *allowMembers) ResolveWorkspaceOrganization(_ context.Context, _ kernel.WorkspaceID) (kernel.OrgID, bool, error) {
	return kernel.OrgID{UUID: a.organizationID}, true, nil
}

type noRoles struct{}

func (noRoles) ListWorkspaceRoleIDs(context.Context, kernel.WorkspaceID, kernel.MemberID) ([]kernel.RoleID, error) {
	return nil, nil
}
func (noRoles) ResolveRoleWorkspace(context.Context, kernel.RoleID) (kernel.WorkspaceID, error) {
	return kernel.WorkspaceID{}, nil
}

type fixedRoleScope struct{ workspaceID uuid.UUID }

func (s fixedRoleScope) ResolveRoleWorkspace(context.Context, kernel.RoleID) (kernel.WorkspaceID, error) {
	return kernel.WorkspaceID{UUID: s.workspaceID}, nil
}

type workspaceRolePermissions struct{ allowed bool }

func (p workspaceRolePermissions) HasWorkspaceRolePermission(context.Context, kernel.WorkspaceID, kernel.MemberID, string) (bool, error) {
	return p.allowed, nil
}

type roleBindingsStub struct{ roles []*govmodel.Role }

func (*roleBindingsStub) Save(context.Context, *govmodel.RoleBinding) error { return nil }
func (s *roleBindingsStub) ListRolesForMember(context.Context, kernel.WorkspaceID, kernel.MemberID) ([]*govmodel.Role, error) {
	return s.roles, nil
}
func (*roleBindingsStub) Exists(context.Context, kernel.RoleID, kernel.MemberID) (bool, error) {
	return false, nil
}
func (*roleBindingsStub) Delete(context.Context, kernel.RoleID, kernel.MemberID) error { return nil }
func (*roleBindingsStub) DeleteByRoleID(context.Context, kernel.RoleID) error          { return nil }

type superAdmin struct{}

func (superAdmin) IsSuperAdmin(context.Context, kernel.MemberID) (bool, error) {
	return true, nil
}

type allowOrgPermissions struct{}

func (allowOrgPermissions) IsOrgMember(context.Context, kernel.OrgID, kernel.MemberID) (bool, error) {
	return true, nil
}
func (allowOrgPermissions) HasOrgPermission(context.Context, kernel.OrgID, kernel.MemberID, string) (bool, error) {
	return true, nil
}

type discardAudit struct{}

func (discardAudit) Log(context.Context, audit.Entry) error { return nil }

func TestPrincipalScopeAdapterUsesAuthoritativeTenantAndGovernanceScopes(t *testing.T) {
	organizationID, workspaceID := uuid.New(), uuid.New()
	adapter := principalScopeAdapter{
		workspaces: &allowMembers{organizationID: organizationID},
		roles:      fixedRoleScope{workspaceID: workspaceID}, organizations: allowOrgPermissions{},
	}
	for _, item := range []struct {
		principalType model.PrincipalType
		principalID   uuid.UUID
		wantWorkspace uuid.UUID
	}{
		{model.PrincipalUser, uuid.New(), uuid.Nil},
		{model.PrincipalWorkspace, workspaceID, workspaceID},
		{model.PrincipalOrganization, organizationID, uuid.Nil},
		{model.PrincipalRole, uuid.New(), workspaceID},
	} {
		scope, err := adapter.Resolve(context.Background(), item.principalType, item.principalID, organizationID)
		require.NoError(t, err)
		require.Equal(t, organizationID, scope.OrganizationID)
		require.Equal(t, item.wantWorkspace, scope.WorkspaceID)
	}
	_, err := adapter.Resolve(context.Background(), model.PrincipalServiceAccount, uuid.New(), organizationID)
	require.ErrorIs(t, err, domain.ErrUnsupportedPrincipal)
}

func TestRegisterValidatesDependencies(t *testing.T) {
	registry := events.NewRegistry()
	base := appctx.Container{
		Config:                   &config.Config{},
		DB:                       &gorm.DB{},
		Transactor:               postgres.NewDirectTransactor(),
		EventBus:                 messaging.NewInMemoryBus(registry),
		PermissionChecker:        allowPermissions{},
		WorkspaceMemberChecker:   &allowMembers{},
		WorkspaceAccessResolver:  &allowMembers{},
		WorkspaceScopeResolver:   &allowMembers{},
		WorkspaceRoleReader:      noRoles{},
		RoleScopeResolver:        noRoles{},
		WorkspaceRolePermissions: workspaceRolePermissions{allowed: true},
		OrgPermissionChecker:     allowOrgPermissions{},
		AuditLogger:              discardAudit{},
		EventRegistry:            registry,
		Engine:                   httpx.NewEngine(),
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
		{name: "workspace access resolver", drop: func(c *appctx.Container) { c.WorkspaceAccessResolver = nil }, want: "requires workspace access resolver"},
		{name: "workspace scope resolver", drop: func(c *appctx.Container) { c.WorkspaceScopeResolver = nil }, want: "requires workspace scope resolver"},
		{name: "workspace role reader", drop: func(c *appctx.Container) { c.WorkspaceRoleReader = nil }, want: "requires workspace role reader"},
		{name: "role scope resolver", drop: func(c *appctx.Container) { c.RoleScopeResolver = nil }, want: "requires role scope resolver"},
		{name: "workspace role permission checker", drop: func(c *appctx.Container) { c.WorkspaceRolePermissions = nil }, want: "requires workspace role permission checker"},
		{name: "organization permission checker", drop: func(c *appctx.Container) { c.OrgPermissionChecker = nil }, want: "requires organization permission checker"},
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

func TestDefaultConfigurationConstructsSharedCursorCodec(t *testing.T) {
	_, secret, _ := transferGatewayDefaults(&appctx.Container{Config: &config.Config{}})

	codec, err := filesystemquery.NewSharedCursorCodec(secret, nil)

	require.NoError(t, err)
	require.NotNil(t, codec)
}

func TestRegisterExposesMetadataRoutesWithoutLocalProvider(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	registry := events.NewRegistry()
	engine := httpx.NewEngine()
	memberAccess := &allowMembers{}
	container := &appctx.Container{
		Config:                   &config.Config{},
		DB:                       db,
		Transactor:               postgres.NewTransactor(db),
		EventBus:                 messaging.NewInMemoryBus(registry),
		PermissionChecker:        allowPermissions{},
		WorkspaceMemberChecker:   memberAccess,
		WorkspaceAccessResolver:  memberAccess,
		WorkspaceScopeResolver:   memberAccess,
		WorkspaceRoleReader:      noRoles{},
		RoleScopeResolver:        noRoles{},
		WorkspaceRolePermissions: workspaceRolePermissions{allowed: true},
		OrgPermissionChecker:     allowOrgPermissions{},
		AuditLogger:              discardAudit{},
		EventRegistry:            registry,
		Engine:                   engine,
	}
	module := New(authn.NewTokenProvider(config.JWTConfig{}))
	require.NoError(t, module.Register(container))

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	require.True(t, routes["GET /api/v1/workspaces/:wsId/files"])
	require.True(t, routes["POST /api/v1/workspaces/:wsId/directories"])
	require.True(t, routes["GET /api/v1/filesystem/namespaces"])
	require.True(t, routes["GET /api/v1/filesystem/namespaces/:namespaceId/children"])
	require.True(t, routes["GET /api/v1/filesystem/shared-with-me"])
	require.True(t, routes["GET /api/v1/filesystem/nodes/:nodeId/effective-access"])
	require.False(t, routes["POST /api/v1/workspaces/:wsId/files"], "upload is deferred to G2")
	require.NotNil(t, container.FilesystemGrantPort)
	repo := persistence.NewRepository(db)
	owner := uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	memberAccess.organizationID = namespace.OrganizationID
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

func TestNamespaceAuthorityRequiresWorkspaceRolePermission(t *testing.T) {
	authority := namespaceAuthorityAdapter{
		workspaces:      &allowMembers{},
		rolePermissions: workspaceRolePermissions{allowed: false},
		organizations:   allowOrgPermissions{},
	}

	allowed, err := authority.HasWorkspaceFileAdmin(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	require.False(t, allowed)
}

func TestWorkspaceNamespaceAdminDoesNotUseSystemSuperAdminBypass(t *testing.T) {
	workspaceID, actorID := uuid.New(), uuid.New()
	bindings := &roleBindingsStub{}
	checker := govquery.NewChecker(bindings, superAdmin{})
	authority := namespaceAuthorityAdapter{
		workspaces:      &allowMembers{},
		rolePermissions: checker,
		organizations:   allowOrgPermissions{},
	}

	allowed, err := authority.HasWorkspaceFileAdmin(context.Background(), workspaceID, actorID)
	require.NoError(t, err)
	require.False(t, allowed)

	bindings.roles = []*govmodel.Role{govmodel.NewRole(
		kernel.WorkspaceID{UUID: workspaceID}, "file-admin", []string{authz.PermFileAdmin}, nil,
	)}
	allowed, err = authority.HasWorkspaceFileAdmin(context.Background(), workspaceID, actorID)
	require.NoError(t, err)
	require.True(t, allowed)
}

var _ authz.PermissionChecker = allowPermissions{}
var _ authz.WorkspaceMemberChecker = (*allowMembers)(nil)
var _ authz.WorkspaceAccessResolver = (*allowMembers)(nil)
var _ authz.WorkspaceRoleReader = noRoles{}
var _ authz.WorkspaceRolePermissionChecker = workspaceRolePermissions{}
var _ authz.OrgPermissionChecker = allowOrgPermissions{}
var _ audit.Logger = discardAudit{}
