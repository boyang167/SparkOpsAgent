package manifest_test

import (
	"os"
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

const validYAML = `id: example
version: "0.1.0"
display_name: Example Plugin
route_prefix: /api/v1/plugins/example
entitlement: plugin:example
menus:
  - title: Example
    path: /plugins/example
    component: plugins/example/page
apis:
  - method: GET
    path: /api/v1/plugins/example/ping
frontend:
  pages:
    - app/(platform)/plugins/example/page.tsx
`

func requireManifestInvalid(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	de, ok := kernel.IsDomainError(err)
	require.True(t, ok)
	require.Equal(t, "PLUGIN_MANIFEST_INVALID", de.Code())
}

func TestParse_ValidExampleYAML(t *testing.T) {
	raw, err := os.ReadFile("../../../../../internal/plugins/example/plugin.yaml")
	require.NoError(t, err)

	m, err := manifest.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "example", m.ID)
	require.Equal(t, "0.1.0", m.Version)
	require.Equal(t, "Example Plugin", m.DisplayName)
	require.Equal(t, "/api/v1/plugins/example", m.RoutePrefix)
	require.Equal(t, "plugin:example", m.Entitlement)
	require.Len(t, m.Menus, 1)
	require.Equal(t, "Example", m.Menus[0].Title)
	require.Equal(t, "/plugins/example", m.Menus[0].Path)
	require.Equal(t, "plugins/example/page", m.Menus[0].Component)
	require.Len(t, m.APIs, 1)
	require.Equal(t, "GET", m.APIs[0].Method)
	require.Equal(t, "/api/v1/plugins/example/ping", m.APIs[0].Path)
	require.Len(t, m.Frontend.Pages, 1)
	require.Equal(t, "app/(platform)/plugins/example/page.tsx", m.Frontend.Pages[0])
}

func TestParse_ValidInlineYAML(t *testing.T) {
	m, err := manifest.Parse([]byte(validYAML))
	require.NoError(t, err)
	require.Equal(t, "example", m.ID)
}

func TestParse_MissingID(t *testing.T) {
	yaml := strings.Replace(validYAML, "id: example\n", "", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidEntitlement(t *testing.T) {
	yaml := strings.Replace(validYAML, "entitlement: plugin:example", "entitlement: delivery", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidAPIPathPrefix(t *testing.T) {
	yaml := strings.Replace(validYAML, "path: /api/v1/plugins/example/ping", "path: /other", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidMenuPathPrefix(t *testing.T) {
	yaml := strings.Replace(validYAML, "path: /plugins/example", "path: /other", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidAPIMethod(t *testing.T) {
	yaml := strings.Replace(validYAML, "method: GET", "method: TRACE", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidAPIMethod_Lowercase(t *testing.T) {
	yaml := strings.Replace(validYAML, "method: GET", "method: get", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidAPIMethod_WithWhitespace(t *testing.T) {
	yaml := strings.Replace(validYAML, "method: GET", `method: " post "`, 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}
