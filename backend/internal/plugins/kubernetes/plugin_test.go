package kubernetes

import (
	"archive/tar"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPluginReadyRouteRegistersWithContext(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(&plugin{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})

	sparkplugin.InstallAllWithContext(engine, sparkplugin.PluginContext{
		Logger:  zap.NewNop(),
		Members: pluginMembers{wsID: wsID, memberID: memberID},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/kubernetes/ready", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"ok":true}`, w.Body.String())
}

func TestPluginSupportsLegacyAndContextEntrypointsOnSeparateEngines(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &plugin{}
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()

	legacyEngine := readyEngine(memberID)
	p.Register(legacyEngine)
	requireReadyStatus(t, legacyEngine, wsID, http.StatusServiceUnavailable)

	contextEngine := readyEngine(memberID)
	p.RegisterWithContext(contextEngine, sparkplugin.PluginContext{
		Logger:  zap.NewNop(),
		Members: pluginMembers{wsID: wsID, memberID: memberID},
	})
	requireReady(t, contextEngine, wsID)
}

func TestPluginImplementsContextPlugin(t *testing.T) {
	var _ sparkplugin.ContextPlugin = (*plugin)(nil)
}

func TestRegisterBundledDescriptor(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)

	registerBundledDescriptor()
	descriptor, ok := sparkplugin.Bundled("kubernetes")
	require.True(t, ok)
	require.Equal(t, ManifestYAML, descriptor.ManifestYAML)
	require.Contains(t, ManifestYAML, "/pods/:pod/terminal-ticket")
	require.Len(t, descriptor.InstalledFiles, 133)
	for _, required := range []string{
		"backend/internal/plugins/kubernetes/plugin.go",
		"backend/internal/plugins/kubernetes/plugin.yaml",
		"backend/internal/plugins/kubernetes/diagnosis/agent.go",
		"backend/internal/plugins/kubernetes/mcp_tools.go",
		"fronted/apps/web/app/api/bff/v1/plugins/kubernetes/[...path]/route.ts",
	} {
		require.Contains(t, descriptor.InstalledFiles, required)
	}
	seen := make(map[string]struct{}, len(descriptor.InstalledFiles))
	for _, file := range descriptor.InstalledFiles {
		_, duplicate := seen[file]
		require.Falsef(t, duplicate, "duplicate installed file %q", file)
		seen[file] = struct{}{}
	}
}

func TestArthasTarContainsBootJar(t *testing.T) {
	f, err := os.Open(filepath.Join("resource", "arthas-bin.tar"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	requireArthasTarContainsBootJar(t, f)
}

func requireArthasTarContainsBootJar(t *testing.T, r io.Reader) {
	t.Helper()

	tr := tar.NewReader(r)
	foundBootJar := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		requireSafeTarHeader(t, header)
		if header.Name == "arthas-bin/arthas-boot.jar" {
			require.Positive(t, header.Size)
			foundBootJar = true
		}
	}
	require.True(t, foundBootJar, "arthas-bin/arthas-boot.jar not found")
}

func requireSafeTarHeader(t *testing.T, header *tar.Header) {
	t.Helper()

	require.NotContains(t, header.Name, `\`)
	require.False(t, strings.HasPrefix(header.Name, "/"), header.Name)
	cleanName := strings.TrimSuffix(header.Name, "/")
	require.NotEmpty(t, cleanName)
	require.Equal(t, path.Clean(cleanName), cleanName)
	for _, segment := range strings.Split(cleanName, "/") {
		require.NotEmpty(t, segment, header.Name)
		require.NotEqual(t, ".", segment, header.Name)
		require.NotEqual(t, "..", segment, header.Name)
	}
	require.NotEqual(t, tar.TypeSymlink, header.Typeflag, header.Name)
	require.NotEqual(t, tar.TypeLink, header.Typeflag, header.Name)
}

func readyEngine(memberID kernel.MemberID) *gin.Engine {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	return engine
}

func requireReady(t *testing.T, engine *gin.Engine, wsID kernel.WorkspaceID) {
	t.Helper()
	requireReadyStatus(t, engine, wsID, http.StatusOK)
}

func requireReadyStatus(t *testing.T, engine *gin.Engine, wsID kernel.WorkspaceID, status int) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/kubernetes/ready", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	engine.ServeHTTP(w, req)
	require.Equal(t, status, w.Code)
	if status == http.StatusOK {
		require.JSONEq(t, `{"ok":true}`, w.Body.String())
	}
}

type pluginMembers struct {
	wsID     kernel.WorkspaceID
	memberID kernel.MemberID
}

func (m pluginMembers) IsWorkspaceMember(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return true, nil
}
