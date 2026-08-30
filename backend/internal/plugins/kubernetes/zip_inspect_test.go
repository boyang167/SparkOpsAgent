package kubernetes

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/install"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/stretchr/testify/require"
)

const (
	inspectZipMaxEntries       = 2000
	inspectZipMaxExpandedBytes = 80 << 20
)

func TestKubernetesBundledFilesBuildRealInspectableZip(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	registerBundledDescriptor()
	descriptor, ok := sparkplugin.Bundled("kubernetes")
	require.True(t, ok)

	repo := repoRootFromCaller(t)
	manifestData := []byte(ManifestYAML)
	parsedManifest, err := manifest.Parse(manifestData)
	require.NoError(t, err)
	require.Equal(t, "kubernetes", parsedManifest.ID)

	requireDescriptorCoversProductionSources(t, repo, descriptor.InstalledFiles, parsedManifest)
	raw := buildKubernetesPluginZip(t, repo, descriptor.InstalledFiles, manifestData)
	requireZipWithinInspectLimits(t, raw)

	inspected, err := install.InspectZip(raw)
	require.NoError(t, err)
	require.Equal(t, "kubernetes", inspected.Manifest.ID)
	require.Contains(t, inspected.BackendFiles, "internal/plugins/kubernetes/plugin.go")
	require.Contains(t, inspected.BackendFiles, "internal/plugins/kubernetes/resource/arthas-bin.tar")
	tarInfo, err := os.Stat(filepath.Join(repo, "backend", "internal", "plugins", "kubernetes", "resource", "arthas-bin.tar"))
	require.NoError(t, err)
	require.Equal(t, int(tarInfo.Size()), len(inspected.BackendFiles["internal/plugins/kubernetes/resource/arthas-bin.tar"]))
	require.Greater(t, len(inspected.BackendFiles["internal/plugins/kubernetes/resource/arthas-bin.tar"]), 19<<20)

	for _, page := range parsedManifest.Frontend.Pages {
		require.Contains(t, inspected.FrontendFiles, page)
		require.Contains(t, inspected.Files, page)
	}
}

func requireDescriptorCoversProductionSources(t *testing.T, repo string, installed []string, m *manifest.Manifest) {
	t.Helper()

	installedSet := make(map[string]struct{}, len(installed))
	for _, rel := range installed {
		requireProductionDescriptorPath(t, rel)
		require.FileExists(t, filepath.Join(repo, filepath.FromSlash(rel)), rel)
		installedSet[rel] = struct{}{}
	}

	for _, rel := range productionKubernetesSources(t, repo) {
		require.Contains(t, installedSet, rel, "descriptor is missing production source %s", rel)
	}
	for _, page := range m.Frontend.Pages {
		require.Contains(t, installedSet, "fronted/apps/web/"+page, "descriptor is missing manifest page %s", page)
	}
}

func requireProductionDescriptorPath(t *testing.T, rel string) {
	t.Helper()

	require.True(t, fs.ValidPath(rel), "descriptor path must be slash-clean: %s", rel)
	require.Equal(t, path.Clean(rel), rel, "descriptor path must be clean: %s", rel)
	require.False(t, strings.Contains(rel, "node_modules/"), rel)
	require.False(t, strings.Contains(rel, "/.next/"), rel)
	require.False(t, strings.HasSuffix(rel, ".tsbuildinfo"), rel)
	require.False(t, strings.HasSuffix(rel, "_test.go"), rel)
	require.False(t, strings.HasSuffix(rel, ".test.ts"), rel)
	require.False(t, strings.HasSuffix(rel, ".test.tsx"), rel)
	require.True(t,
		strings.HasPrefix(rel, "backend/internal/plugins/kubernetes/") ||
			strings.HasPrefix(rel, "fronted/apps/web/app/(platform)/plugins/kubernetes/") ||
			strings.HasPrefix(rel, "fronted/apps/web/src/modules/plugins/kubernetes/"),
		"descriptor path is outside kubernetes production plugin files: %s", rel)
}

func productionKubernetesSources(t *testing.T, repo string) []string {
	t.Helper()

	roots := []string{
		"backend/internal/plugins/kubernetes",
		"fronted/apps/web/app/(platform)/plugins/kubernetes",
		"fronted/apps/web/src/modules/plugins/kubernetes",
	}
	var out []string
	for _, root := range roots {
		base := filepath.Join(repo, filepath.FromSlash(root))
		require.NoError(t, filepath.WalkDir(base, func(abs string, d fs.DirEntry, err error) error {
			require.NoError(t, err)
			if d.IsDir() {
				return nil
			}
			rel := filepath.ToSlash(mustRel(t, repo, abs))
			if isProductionKubernetesSource(rel) {
				out = append(out, rel)
			}
			return nil
		}))
	}
	return out
}

func isProductionKubernetesSource(rel string) bool {
	switch {
	case strings.HasSuffix(rel, "_test.go"),
		strings.HasSuffix(rel, ".test.ts"),
		strings.HasSuffix(rel, ".test.tsx"),
		strings.HasSuffix(rel, ".tsbuildinfo"):
		return false
	case strings.HasSuffix(rel, ".go"),
		strings.HasSuffix(rel, ".yaml"),
		strings.HasSuffix(rel, ".tar"),
		strings.HasSuffix(rel, ".ts"),
		strings.HasSuffix(rel, ".tsx"):
		return true
	default:
		return false
	}
}

func buildKubernetesPluginZip(t *testing.T, repo string, installed []string, manifestData []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	addZipEntry(t, zw, "kubernetes/plugin.yaml", manifestData)
	seen := map[string]struct{}{"kubernetes/plugin.yaml": {}}

	for _, rel := range installed {
		content, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
		require.NoError(t, err, rel)
		zipName := zipNameForInstalledFile(t, rel)
		if _, ok := seen[zipName]; ok {
			t.Fatalf("duplicate zip entry %s", zipName)
		}
		seen[zipName] = struct{}{}
		addZipEntry(t, zw, zipName, content)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func zipNameForInstalledFile(t *testing.T, rel string) string {
	t.Helper()

	switch {
	case strings.HasPrefix(rel, "backend/"):
		return "kubernetes/" + rel
	case strings.HasPrefix(rel, "fronted/apps/web/"):
		return "kubernetes/frontend/" + strings.TrimPrefix(rel, "fronted/apps/web/")
	default:
		t.Fatalf("cannot map installed file into plugin zip: %s", rel)
		return ""
	}
}

func addZipEntry(t *testing.T, zw *zip.Writer, name string, content []byte) {
	t.Helper()

	require.True(t, fs.ValidPath(name), "zip path must be slash-clean: %s", name)
	w, err := zw.Create(name)
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
}

func requireZipWithinInspectLimits(t *testing.T, raw []byte) {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)
	require.LessOrEqual(t, len(zr.File), inspectZipMaxEntries)
	var expanded uint64
	for _, f := range zr.File {
		expanded += f.UncompressedSize64
	}
	require.LessOrEqual(t, expanded, uint64(inspectZipMaxExpandedBytes))
}

func repoRootFromCaller(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(file)
	for {
		if fileExists(filepath.Join(dir, "backend", "go.mod")) && fileExists(filepath.Join(dir, "fronted", "apps", "web", "package.json")) {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "repo root not found from %s", file)
		dir = parent
	}
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()

	rel, err := filepath.Rel(base, target)
	require.NoError(t, err)
	return rel
}
