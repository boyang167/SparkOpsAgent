package openapi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	specopenapi "github.com/sparkops/sparkops-agent/backend/api/openapi"
	filesystemhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestFilesystemOpenAPIPathsMatchHTTPContract(t *testing.T) {
	rootRaw, err := specopenapi.Files.ReadFile("openapi.yaml")
	require.NoError(t, err)
	filesystemRaw, err := specopenapi.Files.ReadFile("filesystem.yaml")
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
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
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
			responses := operation["responses"].(map[string]any)
			for _, status := range []string{"401", "403", "409", "422", "500", "503"} {
				require.Containsf(t, responses, status, "%s %s must document %s", method, path, status)
			}
		}
	}
	assertLocalRefsResolve(t, document, document)
}

func TestFilesystemRoutesDeclareImplementedStableErrors(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))
	paths := document["paths"].(map[string]any)

	expected := map[string][]string{
		"GET /api/v1/workspaces/{wsId}/namespaces":                                     {"400", "401", "403", "404", "500", "503"},
		"GET /api/v1/workspaces/{wsId}/files":                                          {"400", "401", "403", "404", "422", "500", "503"},
		"POST /api/v1/workspaces/{wsId}/directories":                                   {"400", "401", "403", "404", "409", "415", "422", "500", "503"},
		"PATCH /api/v1/workspaces/{wsId}/files/{nodeId}/name":                          {"400", "401", "403", "404", "409", "415", "422", "500", "503"},
		"POST /api/v1/workspaces/{wsId}/files/{nodeId}/move":                           {"400", "401", "403", "404", "409", "415", "422", "500", "503"},
		"POST /api/v1/workspaces/{wsId}/files/{nodeId}/copy":                           {"400", "401", "403", "404", "409", "415", "422", "500", "503"},
		"DELETE /api/v1/workspaces/{wsId}/files/{nodeId}":                              {"400", "401", "403", "404", "409", "422", "500", "503"},
		"GET /api/v1/workspaces/{wsId}/files/{nodeId}/revisions":                       {"400", "401", "403", "404", "422", "500", "503"},
		"POST /api/v1/workspaces/{wsId}/files/{nodeId}/revisions/{revisionId}/restore": {"400", "401", "403", "404", "409", "422", "500", "503"},
	}
	for route, statuses := range expected {
		parts := strings.SplitN(route, " ", 2)
		operation := paths[parts[1]].(map[string]any)[strings.ToLower(parts[0])].(map[string]any)
		responses := operation["responses"].(map[string]any)
		actualStatuses := make([]string, 0, len(responses))
		for status := range responses {
			if status >= "400" {
				actualStatuses = append(actualStatuses, status)
			}
		}
		require.ElementsMatchf(t, statuses, actualStatuses, "%s error response set", route)
		for _, status := range statuses {
			response, ok := responses[status]
			require.Truef(t, ok, "%s must declare %s", route, status)
			require.Equal(t, "#/components/schemas/ErrorResponse", responseSchemaRef(document, response), "%s %s", route, status)
		}
	}
}

func TestErrorResponseAllowsAllFilesystemMiddlewareCodes(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))
	components := document["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	errorResponse := schemas["ErrorResponse"].(map[string]any)
	properties := errorResponse["properties"].(map[string]any)
	data := properties["data"].(map[string]any)
	dataProperties := data["properties"].(map[string]any)
	errorCode := dataProperties["error_code"].(map[string]any)
	rawEnum := errorCode["enum"].([]any)
	allowed := make([]string, 0, len(rawEnum))
	for _, value := range rawEnum {
		allowed = append(allowed, value.(string))
	}
	require.Subset(t, allowed, []string{
		"AUTH_UNAUTHORIZED",
		"AUTH_INVALID_TOKEN",
		"TENANT_WORKSPACE_REQUIRED",
		"TENANT_WORKSPACE_INVALID",
		"TENANT_NOT_WORKSPACE_MEMBER",
		"ENTITLEMENT_MODULE_DISABLED",
		"AUTH_FORBIDDEN",
		"INTERNAL_ERROR",
	})
}

func TestRootOpenAPILoadsExternalRefsAndValidates(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile("openapi.yaml")
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))
}

type openAPIPermissions struct{}

func (openAPIPermissions) RequirePermission(context.Context, kernel.WorkspaceID, kernel.MemberID, string) error {
	return nil
}
func (openAPIPermissions) HasPermission(context.Context, kernel.WorkspaceID, kernel.MemberID, string) (bool, error) {
	return true, nil
}

type openAPIMembers struct{}

func (openAPIMembers) IsWorkspaceMember(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return true, nil
}

func TestFilesystemSpecificationExactlyMatchesRouter(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var fragment map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &fragment))
	specPaths := fragment["paths"].(map[string]any)
	spec := map[string]bool{}
	for path, rawItem := range specPaths {
		for method := range rawItem.(map[string]any) {
			spec[strings.ToUpper(method)+" "+path] = true
		}
	}

	engine := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	filesystemhttp.RegisterRoutes(engine, pass, pass, openAPIMembers{}, openAPIPermissions{}, &filesystemhttp.Handler{})
	actual := map[string]bool{}
	for _, route := range engine.Routes() {
		path := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(route.Path, ":wsId", "{wsId}"), ":nodeId", "{nodeId}"), ":revisionId", "{revisionId}")
		actual[route.Method+" "+path] = true
	}
	require.Equal(t, spec, actual)
}

var _ authz.PermissionChecker = openAPIPermissions{}
var _ authz.WorkspaceMemberChecker = openAPIMembers{}

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

func responseSchemaRef(root map[string]any, response any) string {
	value := response.(map[string]any)
	if ref, _ := value["$ref"].(string); strings.HasPrefix(ref, "#/") {
		value = resolveLocalRef(root, ref).(map[string]any)
	}
	content := value["content"].(map[string]any)
	media := content["application/json"].(map[string]any)
	schema := media["schema"].(map[string]any)
	ref, _ := schema["$ref"].(string)
	return ref
}
