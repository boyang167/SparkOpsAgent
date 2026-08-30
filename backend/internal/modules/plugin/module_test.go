package plugin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type systemRoleChecker struct {
	super bool
}

func (c systemRoleChecker) IsSuperAdmin(context.Context, kernel.MemberID) (bool, error) {
	return c.super, nil
}

type workspaceMemberChecker struct{ member bool }

func (c workspaceMemberChecker) IsWorkspaceMember(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return c.member, nil
}

type permissionChecker struct{ allowed bool }

func (c permissionChecker) RequirePermission(
	context.Context,
	kernel.WorkspaceID,
	kernel.MemberID,
	string,
) error {
	if c.allowed {
		return nil
	}
	return kernel.NewDomainError("AUTH_FORBIDDEN", "forbidden")
}

func (c permissionChecker) HasPermission(
	context.Context,
	kernel.WorkspaceID,
	kernel.MemberID,
	string,
) (bool, error) {
	return c.allowed, nil
}

func TestModuleRequiresConfiguredCleanGitPublisher(t *testing.T) {
	t.Setenv("PLUGIN_GIT_ROOT", "")
	db := openModuleDB(t)
	module := plugin.New(authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret"}))

	err := module.Register(&appctx.Container{
		Context:           context.Background(),
		DB:                db,
		Engine:            gin.New(),
		SystemRoleChecker: systemRoleChecker{super: true},
	})

	require.ErrorContains(t, err, "PLUGIN_GIT_ROOT is required")
}

func TestAllPlatformPluginRoutesRequireSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	cmd := exec.Command("git", "init", root)
	require.NoError(t, cmd.Run())
	t.Setenv("PLUGIN_GIT_ROOT", root)
	db := openModuleDB(t)
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret"})
	module := plugin.New(tokens)
	engine := gin.New()
	require.NoError(t, module.Register(&appctx.Container{
		Context:           context.Background(),
		DB:                db,
		Engine:            engine,
		SystemRoleChecker: systemRoleChecker{super: false},
	}))
	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/admin/plugins/install"},
		{http.MethodGet, "/api/v1/admin/plugins"},
		{http.MethodGet, "/api/v1/admin/plugins/example"},
		{http.MethodPost, "/api/v1/admin/plugins/example/merged"},
		{http.MethodPost, "/api/v1/admin/plugins/example/deployed"},
		{http.MethodPost, "/api/v1/admin/plugins/example/uninstall"},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusForbidden, response.Code)
			require.Contains(t, response.Body.String(), "AUTH_FORBIDDEN")
		})
	}
}

func TestModuleName(t *testing.T) {
	require.Equal(t, "plugin", plugin.New(nil).Name())
}

func TestWorkspacePluginRoutesRequireMembershipAndGovernanceAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	cmd := exec.Command("git", "init", root)
	require.NoError(t, cmd.Run())
	t.Setenv("PLUGIN_GIT_ROOT", root)
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret"})
	engine := gin.New()
	require.NoError(t, plugin.New(tokens).Register(&appctx.Container{
		Context:                context.Background(),
		DB:                     openModuleDB(t),
		Engine:                 engine,
		SystemRoleChecker:      systemRoleChecker{super: true},
		WorkspaceMemberChecker: workspaceMemberChecker{member: true},
		PermissionChecker:      permissionChecker{allowed: false},
	}))
	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	wsID := kernel.NewWorkspaceID().String()

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/workspaces/" + wsID + "/plugins"},
		{http.MethodPost, "/api/v1/workspaces/" + wsID + "/plugins/example/enable"},
		{http.MethodPost, "/api/v1/workspaces/" + wsID + "/plugins/example/disable"},
	} {
		request := httptest.NewRequest(route.method, route.path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	}
}

func openModuleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin").Error)
	return db
}
