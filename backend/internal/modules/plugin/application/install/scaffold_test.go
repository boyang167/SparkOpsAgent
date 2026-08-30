package install

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/stretchr/testify/require"
)

func TestScaffoldTemplateProducesValidInspectablePlugin(t *testing.T) {
	files, err := GenerateScaffold("demo", "Demo")
	require.NoError(t, err)

	yamlContent, ok := files["demo/plugin.yaml"]
	require.True(t, ok, "expected demo/plugin.yaml")
	m, err := manifest.Parse(yamlContent)
	require.NoError(t, err)
	require.Equal(t, "demo", m.ID)
	require.NoError(t, m.Validate())

	pluginGo, ok := files["demo/backend/plugin.go"]
	require.True(t, ok, "expected demo/backend/plugin.go")
	require.Contains(t, string(pluginGo), "Register")
	require.True(t, FileDeclaresRegisterEngine(pluginGo), "plugin.go must declare Register(*gin.Engine)")

	pagePath := "demo/frontend/app/(platform)/plugins/demo/page.tsx"
	_, ok = files[pagePath]
	require.True(t, ok, "expected %s", pagePath)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	inspected, err := InspectZip(buf.Bytes())
	require.NoError(t, err)
	require.Equal(t, "demo", inspected.Manifest.ID)
}
