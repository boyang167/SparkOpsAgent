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

func requireAPIReadOnly(t *testing.T, apis []manifest.APISpec, method, path string, readOnly bool) {
	t.Helper()
	for _, api := range apis {
		if api.Method == method && api.Path == path {
			require.Equal(t, readOnly, api.ReadOnly)
			return
		}
	}
	require.Failf(t, "missing api", "%s %s", method, path)
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

func TestParse_KubernetesRecordsGETReliesOnBinderReadOnlyDefault(t *testing.T) {
	raw, err := os.ReadFile("../../../../../internal/plugins/kubernetes/plugin.yaml")
	require.NoError(t, err)

	m, err := manifest.Parse(raw)
	require.NoError(t, err)
	requireAPIReadOnly(t, m.APIs, "GET", "/api/v1/plugins/kubernetes/records", false)
	requireAPIReadOnly(t, m.APIs, "GET", "/api/v1/plugins/kubernetes/records/:recordId", false)
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

func TestParse_ValidANYWildcard(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - method: GET\n    path: /api/v1/plugins/example/ping",
		"  - method: ANY\n    path: /api/v1/plugins/example/proxy/*", 1)
	m, err := manifest.Parse([]byte(yaml))
	require.NoError(t, err)
	require.Equal(t, "ANY", m.APIs[0].Method)
	require.Equal(t, "/api/v1/plugins/example/proxy/*", m.APIs[0].Path)
}

func TestParse_ValidExactPOSTReadOnly(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - method: GET\n    path: /api/v1/plugins/example/ping",
		"  - method: POST\n    path: /api/v1/plugins/example/metrics\n    read_only: true", 1)
	m, err := manifest.Parse([]byte(yaml))
	require.NoError(t, err)
	require.True(t, m.APIs[0].ReadOnly)
}

func TestParse_InvalidANYReadOnly(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - method: GET\n    path: /api/v1/plugins/example/ping",
		"  - method: ANY\n    path: /api/v1/plugins/example/proxy/*\n    read_only: true", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidPUTReadOnly(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - method: GET\n    path: /api/v1/plugins/example/ping",
		"  - method: PUT\n    path: /api/v1/plugins/example/ping\n    read_only: true", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidPOSTWildcardReadOnly(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - method: GET\n    path: /api/v1/plugins/example/ping",
		"  - method: POST\n    path: /api/v1/plugins/example/proxy/*\n    read_only: true", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidPOSTInternalWildcardReadOnly(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - method: GET\n    path: /api/v1/plugins/example/ping",
		"  - method: POST\n    path: /api/v1/plugins/example/foo/*/metrics\n    read_only: true", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidANYExactPath(t *testing.T) {
	yaml := strings.Replace(validYAML, "method: GET", "method: ANY", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidParent(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - title: Example\n    path: /plugins/example\n    component: plugins/example/page",
		"  - title: Example\n    path: /plugins/example\n    component: plugins/example/page\n    parent: /plugins/example/missing", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_ValidTwoLevelParent(t *testing.T) {
	yaml := strings.Replace(validYAML,
		`menus:
  - title: Example
    path: /plugins/example
    component: plugins/example/page`,
		`menus:
  - title: Root
    path: /plugins/example
    component: plugins/example/page
  - title: Child
    path: /plugins/example/child
    component: plugins/example/child
    parent: /plugins/example`, 1)
	m, err := manifest.Parse([]byte(yaml))
	require.NoError(t, err)
	require.Len(t, m.Menus, 2)
	require.Equal(t, "/plugins/example", m.Menus[1].Parent)
}

func TestParse_InvalidAssetsFrontend(t *testing.T) {
	yaml := validYAML + `assets:
  - frontend/bundle.tar
`
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidParentSelfLoop(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - title: Example\n    path: /plugins/example\n    component: plugins/example/page",
		"  - title: Example\n    path: /plugins/example\n    component: plugins/example/page\n    parent: /plugins/example", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidParentChainTooDeep(t *testing.T) {
	yaml := strings.Replace(validYAML,
		`menus:
  - title: Example
    path: /plugins/example
    component: plugins/example/page`,
		`menus:
  - title: L1
    path: /plugins/example
    component: plugins/example/page
  - title: L2
    path: /plugins/example/l2
    component: plugins/example/l2
    parent: /plugins/example
  - title: L3
    path: /plugins/example/l2/l3
    component: plugins/example/l2/l3
    parent: /plugins/example/l2
  - title: L4
    path: /plugins/example/l2/l3/l4
    component: plugins/example/l2/l3/l4
    parent: /plugins/example/l2/l3`, 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_ValidAssets(t *testing.T) {
	yaml := validYAML + `assets:
  - backend/plugin/kubernetes/arthas-bin.tar
`
	m, err := manifest.Parse([]byte(yaml))
	require.NoError(t, err)
	require.Equal(t, []string{"backend/plugin/kubernetes/arthas-bin.tar"}, m.Assets)
}

func TestParse_InvalidAssetsPathTraversal(t *testing.T) {
	yaml := validYAML + `assets:
  - backend/../secret.tar
`
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}

func TestParse_InvalidANYOutsideRoutePrefix(t *testing.T) {
	yaml := strings.Replace(validYAML,
		"  - method: GET\n    path: /api/v1/plugins/example/ping",
		"  - method: ANY\n    path: /api/v1/other/*", 1)
	_, err := manifest.Parse([]byte(yaml))
	requireManifestInvalid(t, err)
}
