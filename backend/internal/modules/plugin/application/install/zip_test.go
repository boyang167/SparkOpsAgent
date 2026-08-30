package install

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
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

const validPluginYAMLWithTarAsset = validPluginYAML + `assets:
  - backend/resource/arthas-bin.tar
`

const validPluginGo = `package example

import "github.com/gin-gonic/gin"

type plugin struct{}

func (p *plugin) Register(engine *gin.Engine) {}
`

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	entries := make([]zipEntry, 0, len(files))
	for name, content := range files {
		entries = append(entries, zipEntry{name: name, content: content})
	}
	return makeZipEntries(t, entries)
}

type zipEntry struct {
	name    string
	content string
}

func makeZipEntries(t *testing.T, entries []zipEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range entries {
		w, err := zw.Create(entry.name)
		require.NoError(t, err)
		_, err = w.Write([]byte(entry.content))
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
	require.Equal(t, validPluginGo, string(inspected.BackendFiles["plugin.go"]))
	require.Equal(t, "export default function Page(){return null}", string(inspected.FrontendFiles["app/(platform)/plugins/example/page.tsx"]))
	require.NotContains(t, inspected.Files, "plugin.yaml")
}

func TestInspectZipRejectsMoreThanTwoThousandEntries(t *testing.T) {
	_, err := InspectZip(makeSizedZip(t, nil, 1998))

	requireZipInvalid(t, err)
}

func TestInspectZipAcceptsTwoThousandEntries(t *testing.T) {
	_, err := InspectZip(makeSizedZip(t, nil, 1997))

	require.NoError(t, err)
}

func TestInspectZipRejectsFileLargerThanFiftyMiB(t *testing.T) {
	_, err := InspectZip(makeSizedZip(t, []int64{50<<20 + 1}, 0))

	requireZipInvalid(t, err)
}

func TestInspectZipRejectsMoreThanEightyMiBUncompressed(t *testing.T) {
	_, err := InspectZip(makeSizedZip(t, []int64{
		9 << 20, 9 << 20, 9 << 20, 9 << 20, 9 << 20, 9 << 20, 9 << 20, 9 << 20, 9 << 20,
	}, 0))

	requireZipInvalid(t, err)
}

func TestInspectZipAcceptsMoreThanFiftyMiBUncompressed(t *testing.T) {
	_, err := InspectZip(makeSizedZip(t, []int64{
		9 << 20, 9 << 20, 9 << 20, 9 << 20, 9 << 20, 9 << 20,
	}, 0))

	require.NoError(t, err)
}

func TestInspectZipAcceptsDeclaredTarUnderBackend(t *testing.T) {
	files := validZipFiles()
	files["example/plugin.yaml"] = validPluginYAMLWithTarAsset
	files["example/backend/resource/arthas-bin.tar"] = "tar content"

	inspected, err := InspectZip(makeZip(t, files))

	require.NoError(t, err)
	require.Equal(t, "tar content", string(inspected.BackendFiles["resource/arthas-bin.tar"]))
}

func TestInspectZipRejectsUndeclaredTar(t *testing.T) {
	tests := map[string]struct {
		manifest string
		entry    string
	}{
		"backend tar absent from assets": {
			manifest: validPluginYAML,
			entry:    "example/backend/resource/arthas-bin.tar",
		},
		"backend tar path differs from asset": {
			manifest: validPluginYAMLWithTarAsset,
			entry:    "example/backend/other/arthas-bin.tar",
		},
		"frontend tar is not a backend asset": {
			manifest: validPluginYAMLWithTarAsset,
			entry:    "example/frontend/app/(platform)/plugins/example/arthas-bin.tar",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			files := validZipFiles()
			files["example/plugin.yaml"] = tc.manifest
			files[tc.entry] = "tar content"

			_, err := InspectZip(makeZip(t, files))

			requireZipInvalid(t, err)
		})
	}
}

func TestInspectZipRejectsDuplicateRawEntryNames(t *testing.T) {
	_, err := InspectZip(makeZipEntries(t, []zipEntry{
		{name: "example/plugin.yaml", content: validPluginYAML},
		{name: "example/backend/plugin.go", content: validPluginGo},
		{name: "example/backend/plugin.go", content: `package example

type plugin struct{}
`},
		{name: "example/frontend/app/(platform)/plugins/example/page.tsx", content: "export default function Page(){return null}"},
	}))

	requireZipInvalid(t, err)
}

func TestInspectZipRejectsBackendFrontendMappedKeyCollision(t *testing.T) {
	files := validZipFiles()
	files["example/backend/app/(platform)/plugins/example/page.tsx"] = "backend page"

	_, err := InspectZip(makeZip(t, files))

	requireZipInvalid(t, err)
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

func requireZipInvalid(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)
	de, ok := kernel.IsDomainError(err)
	require.True(t, ok)
	require.Equal(t, "PLUGIN_ZIP_INVALID", de.Code())
}

type repeatedByteReader struct{}

func (repeatedByteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

func makeSizedZip(t *testing.T, sizes []int64, extraEntries int) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	base := []zipEntry{
		{name: "example/plugin.yaml", content: validPluginYAML},
		{name: "example/backend/plugin.go", content: validPluginGo},
		{name: "example/frontend/app/(platform)/plugins/example/page.tsx", content: "export default function Page(){return null}"},
	}
	for _, entry := range base {
		part, err := writer.Create(entry.name)
		require.NoError(t, err)
		_, err = part.Write([]byte(entry.content))
		require.NoError(t, err)
	}
	for i := 0; i < extraEntries; i++ {
		_, err := writer.Create(fmt.Sprintf("example/backend/dir-%04d/", i))
		require.NoError(t, err)
	}
	for i, size := range sizes {
		part, err := writer.Create(fmt.Sprintf("example/backend/data-%02d.txt", i))
		require.NoError(t, err)
		_, err = io.CopyN(part, repeatedByteReader{}, size)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
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

func TestInspectZipRejectsMissingFrontendPageAndUnsafeFrontendPath(t *testing.T) {
	t.Run("manifest page missing from zip", func(t *testing.T) {
		files := validZipFiles()
		delete(files, "example/frontend/app/(platform)/plugins/example/page.tsx")

		_, err := InspectZip(makeZip(t, files))
		requireZipInvalid(t, err)
	})

	t.Run("frontend file outside plugin directories", func(t *testing.T) {
		files := validZipFiles()
		files["example/frontend/app/layout.tsx"] = "export default function Root(){return null}"

		_, err := InspectZip(makeZip(t, files))
		requireZipInvalid(t, err)
	})
}

func TestInspectZipRejectsForbiddenBCImports(t *testing.T) {
	files := validZipFiles()
	files["example/backend/plugin.go"] = `package example

import (
	"github.com/gin-gonic/gin"
	_ "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
)

type plugin struct{}

func (p *plugin) Register(engine *gin.Engine) {}
`

	_, err := InspectZip(makeZip(t, files))
	requireZipInvalid(t, err)
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
