package openapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFilesystemOpenAPIPathsMatchHTTPContract(t *testing.T) {
	rootRaw, err := Files.ReadFile("openapi.yaml")
	require.NoError(t, err)
	filesystemRaw, err := Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)

	var root, filesystem map[string]any
	require.NoError(t, yaml.Unmarshal(rootRaw, &root))
	require.NoError(t, yaml.Unmarshal(filesystemRaw, &filesystem))

	rootPaths := root["paths"].(map[string]any)
	fragmentPaths := filesystem["paths"].(map[string]any)
	expected := map[string]string{
		"/api/v1/workspaces/{wsId}/namespaces":                                    "get",
		"/api/v1/workspaces/{wsId}/files":                                         "get",
		"/api/v1/workspaces/{wsId}/directories":                                   "post",
		"/api/v1/workspaces/{wsId}/files/{nodeId}":                                "delete",
		"/api/v1/workspaces/{wsId}/files/{nodeId}/name":                           "patch",
		"/api/v1/workspaces/{wsId}/files/{nodeId}/move":                           "post",
		"/api/v1/workspaces/{wsId}/files/{nodeId}/copy":                           "post",
		"/api/v1/workspaces/{wsId}/files/{nodeId}/revisions":                      "get",
		"/api/v1/workspaces/{wsId}/files/{nodeId}/revisions/{revisionId}/restore": "post",
	}
	for path, method := range expected {
		require.Contains(t, rootPaths, path)
		item, ok := fragmentPaths[path].(map[string]any)
		require.Truef(t, ok, "path item %s", path)
		require.Contains(t, item, method)
	}

	filesPath := fragmentPaths["/api/v1/workspaces/{wsId}/files"].(map[string]any)
	require.NotContains(t, filesPath, "post", "binary upload is deferred to G2")
	specText := strings.ToLower(string(filesystemRaw))
	require.NotContains(t, specText, "application/octet-stream")
	require.NotContains(t, specText, "format: binary")
	require.Contains(t, specText, "deferred to the g2 transfer gateway")
}

func TestFilesystemOpenAPIIdempotencyAndLocalRefs(t *testing.T) {
	raw, err := Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))

	paths := document["paths"].(map[string]any)
	for path, rawItem := range paths {
		item := rawItem.(map[string]any)
		for method, rawOperation := range item {
			operation := rawOperation.(map[string]any)
			if method == "get" {
				continue
			}
			require.Truef(t, operationHasRef(operation, "#/components/parameters/IdempotencyKey"), "%s %s must require Idempotency-Key", method, path)
		}
	}
	assertLocalRefsResolve(t, document, document)
}

func operationHasRef(operation map[string]any, ref string) bool {
	parameters, _ := operation["parameters"].([]any)
	for _, parameter := range parameters {
		value, _ := parameter.(map[string]any)
		if value["$ref"] == ref {
			return true
		}
	}
	return false
}

func assertLocalRefsResolve(t *testing.T, root map[string]any, value any) {
	t.Helper()
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if key == "$ref" {
				ref, _ := child.(string)
				if strings.HasPrefix(ref, "#/") {
					require.NotNilf(t, resolveLocalRef(root, ref), "unresolved ref %s", ref)
				}
				continue
			}
			assertLocalRefsResolve(t, root, child)
		}
	case []any:
		for _, child := range current {
			assertLocalRefsResolve(t, root, child)
		}
	}
}

func resolveLocalRef(root map[string]any, ref string) any {
	var current any = root
	for _, token := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = object[token]
		if !ok {
			return nil
		}
	}
	return current
}
