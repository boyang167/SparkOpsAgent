package install

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const validPluginYAML = `id: example
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

const validPluginGo = `package example

import "github.com/gin-gonic/gin"

type plugin struct{}

func (p *plugin) Register(engine *gin.Engine) {}
`

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func validZipFiles() map[string]string {
	return map[string]string{
		"example/plugin.yaml":                                      validPluginYAML,
		"example/backend/plugin.go":                                validPluginGo,
		"example/frontend/app/(platform)/plugins/example/page.tsx": "export default function Page(){return null}",
	}
}

func TestInspectZipAcceptsValidPluginAndMapsFiles(t *testing.T) {
	inspected, err := InspectZip(makeZip(t, validZipFiles()))
	require.NoError(t, err)

	require.Equal(t, "example", inspected.Manifest.ID)
	require.Equal(t, validPluginGo, string(inspected.Files["plugin.go"]))
	require.Equal(t, "export default function Page(){return null}", string(inspected.Files["app/(platform)/plugins/example/page.tsx"]))
	require.NotContains(t, inspected.Files, "plugin.yaml")
}

func TestInspectZipRejectsUnsafeEntryNames(t *testing.T) {
	tests := []string{
		"",
		"../plugin.yaml",
		"example/../../plugin.yaml",
		"/example/plugin.yaml",
		`C:/example/plugin.yaml`,
		`C:\example\plugin.yaml`,
		`example\plugin.yaml`,
		`example/%2e%2e/plugin.yaml`,
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			files := validZipFiles()
			files[name] = "unsafe"

			_, err := InspectZip(makeZip(t, files))
			require.Error(t, err)
		})
	}
}

func TestInspectZipRejectsScriptAndBinaryExtensions(t *testing.T) {
	for _, name := range []string{
		"example/backend/install.sh",
		"example/backend/tool.exe",
		"example/backend/run.bat",
		"example/backend/run.cmd",
		"example/backend/run.ps1",
		"example/backend/lib.dll",
		"example/backend/lib.so",
	} {
		t.Run(name, func(t *testing.T) {
			files := validZipFiles()
			files[name] = "blocked"

			_, err := InspectZip(makeZip(t, files))
			require.Error(t, err)
		})
	}
}

func TestInspectZipRequiresManifestAndTopLevelIDMatch(t *testing.T) {
	t.Run("missing manifest", func(t *testing.T) {
		files := validZipFiles()
		delete(files, "example/plugin.yaml")

		_, err := InspectZip(makeZip(t, files))
		require.Error(t, err)
	})

	t.Run("manifest id differs from top level directory", func(t *testing.T) {
		files := validZipFiles()
		files["example/plugin.yaml"] = strings.Replace(validPluginYAML, "id: example", "id: other", 1)

		_, err := InspectZip(makeZip(t, files))
		require.Error(t, err)
	})
}

func TestInspectZipRequiresBackendPluginGoWithRegisterEngine(t *testing.T) {
	t.Run("missing backend plugin go", func(t *testing.T) {
		files := validZipFiles()
		delete(files, "example/backend/plugin.go")

		_, err := InspectZip(makeZip(t, files))
		require.Error(t, err)
	})

	t.Run("plugin go without register engine", func(t *testing.T) {
		files := validZipFiles()
		files["example/backend/nested/plugin.go"] = `package example

type plugin struct{}

func (p *plugin) Register() {}
`
		delete(files, "example/backend/plugin.go")

		_, err := InspectZip(makeZip(t, files))
		require.Error(t, err)
	})

	t.Run("nested backend plugin go is accepted", func(t *testing.T) {
		files := validZipFiles()
		files["example/backend/nested/plugin.go"] = validPluginGo
		delete(files, "example/backend/plugin.go")

		inspected, err := InspectZip(makeZip(t, files))
		require.NoError(t, err)
		require.Contains(t, inspected.Files, "nested/plugin.go")
	})
}
