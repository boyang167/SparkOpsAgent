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
		"/api/v1/workspaces/{wsId}/transfers":                                     "post",
		"/api/v1/workspaces/{wsId}/transfers/{sessionId}":                         "get",
		"/api/v1/workspaces/{wsId}/storage-providers":                             "get",
		"/api/v1/workspaces/{wsId}/storage-pools":                                 "post",
		"/api/v1/workspaces/{wsId}/storage-bindings":                              "get",
		"/api/v1/workspaces/{wsId}/lifecycle-policies":                            "get",
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
	require.Contains(t, specText, "g2 transfer gateway")
}

func TestFilesystemListOpenAPIDocumentsTypeAndQueryFilters(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))

	paths := document["paths"].(map[string]any)
	item := paths["/api/v1/workspaces/{wsId}/files"].(map[string]any)
	get := item["get"].(map[string]any)
	parameters := get["parameters"].([]any)
	names := make([]string, 0, len(parameters))
	for _, rawParameter := range parameters {
		parameter := rawParameter.(map[string]any)
		if name, ok := parameter["name"].(string); ok {
			names = append(names, name)
		}
	}
	require.Contains(t, names, "type")
	require.Contains(t, names, "q")
}

func TestFilesystemGovernanceOpenAPIDocumentsContextHeaders(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))

	paths := document["paths"].(map[string]any)
	for _, path := range []string{
		"/api/v1/filesystem/namespaces",
		"/api/v1/filesystem/namespaces/{namespaceId}/children",
		"/api/v1/filesystem/shared-with-me",
		"/api/v1/filesystem/nodes/{nodeId}/effective-access",
	} {
		operation := paths[path].(map[string]any)["get"].(map[string]any)
		require.True(t, operationHasRef(operation, "#/components/parameters/OrganizationHeader"), path)
		require.True(t, operationHasRef(operation, "#/components/parameters/WorkspaceHeader"), path)
	}
	components := document["components"].(map[string]any)
	parameters := components["parameters"].(map[string]any)
	require.Equal(t, true, parameters["OrganizationHeader"].(map[string]any)["required"])
	require.Equal(t, false, parameters["WorkspaceHeader"].(map[string]any)["required"])
}

func TestFilesystemChildrenOpenAPIDocumentsBoundedOpaqueAuthorizationCursor(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))

	operation := document["paths"].(map[string]any)["/api/v1/filesystem/namespaces/{namespaceId}/children"].(map[string]any)["get"].(map[string]any)
	require.True(t, operationHasRef(operation, "#/components/parameters/ChildrenCursor"))
	require.Contains(t, strings.ToLower(operation["description"].(string)), "256")
	require.Contains(t, operation["responses"].(map[string]any), "503")
	cursor := document["components"].(map[string]any)["parameters"].(map[string]any)["ChildrenCursor"].(map[string]any)
	description := strings.ToLower(cursor["description"].(string))
	require.Contains(t, description, "namespace")
	require.Contains(t, description, "parent")
	require.Contains(t, description, "actor")
	require.Contains(t, description, "filter")
	require.NotContains(t, cursor["schema"].(map[string]any), "format")
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

func TestFilesystemGrantCompatibilityAndRejectReasonSchemas(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	legacy := schemas["CreateResourceGrantRequest"].(map[string]any)
	require.ElementsMatch(t, []any{"grantee_id", "permission", "effect"}, legacy["required"].([]any))
	effect := legacy["properties"].(map[string]any)["effect"].(map[string]any)
	require.ElementsMatch(t, []any{"ALLOW", "DENY"}, effect["enum"].([]any))
	typedEffect := schemas["CreateTypedGrantRequest"].(map[string]any)["properties"].(map[string]any)["effect"].(map[string]any)
	require.ElementsMatch(t, []any{"ALLOW", "DENY"}, typedEffect["enum"].([]any))
	shareRequest := schemas["ShareRequest"].(map[string]any)
	require.Contains(t, shareRequest["required"].([]any), "effect")
	require.ElementsMatch(t, []any{"ALLOW", "DENY"}, shareRequest["properties"].(map[string]any)["effect"].(map[string]any)["enum"].([]any))
	exportTask := schemas["ExportTask"].(map[string]any)
	require.Contains(t, exportTask["required"].([]any), "classification")
	reason := schemas["RejectShareRequest"].(map[string]any)["properties"].(map[string]any)["reason"].(map[string]any)
	require.EqualValues(t, 1, reason["minLength"])
	require.EqualValues(t, 1000, reason["maxLength"])
}

func TestFilesystemGovernancePageSchemasMatchRuntimeResponses(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))

	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	artifactPage := schemas["ArtifactPage"].(map[string]any)
	require.ElementsMatch(t, []any{"items", "next_cursor"}, artifactPage["required"].([]any))

	resourceGrantPage := schemas["ResourceGrantPage"].(map[string]any)
	require.ElementsMatch(
		t,
		[]any{"items", "next_cursor", "truncated"},
		resourceGrantPage["required"].([]any),
	)
	require.Contains(t, resourceGrantPage["properties"].(map[string]any), "next_cursor")
	legacyResourceGrantPage := schemas["LegacyResourceGrantPage"].(map[string]any)
	require.ElementsMatch(t, []any{"items"}, legacyResourceGrantPage["required"].([]any))

	shareRequestPage := schemas["ShareRequestPage"].(map[string]any)
	require.ElementsMatch(
		t,
		[]any{"items", "next_cursor", "truncated", "total_count"},
		shareRequestPage["required"].([]any),
	)
	typedShareProperties := schemas["CreateTypedGrantRequest"].(map[string]any)["properties"].(map[string]any)
	require.Contains(t, typedShareProperties, "reshare")
	require.Contains(t, typedShareProperties, "source_grant_ids")

	paths := document["paths"].(map[string]any)
	for _, path := range []string{
		"/api/v1/filesystem/nodes/{nodeId}/grants",
		"/api/v1/filesystem/nodes/{nodeId}/share-requests",
	} {
		operation := paths[path].(map[string]any)["get"].(map[string]any)
		names := make([]string, 0)
		for _, value := range operation["parameters"].([]any) {
			parameter := value.(map[string]any)
			if name, ok := parameter["name"].(string); ok {
				names = append(names, name)
			}
		}
		require.Contains(t, names, "cursor")
		require.Contains(t, names, "limit")
	}
}

func TestFilesystemResourceActionIncludesDownload(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))

	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	resourceAction := schemas["ResourceAction"].(map[string]any)
	require.ElementsMatch(t,
		[]any{"READ", "DOWNLOAD", "WRITE", "DELETE", "SHARE", "ADMIN"},
		resourceAction["enum"].([]any),
	)
}

func TestSharedWithMeOpenAPIDocumentsSourceProjectionAndFilter(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))

	paths := document["paths"].(map[string]any)
	operation := paths["/api/v1/filesystem/shared-with-me"].(map[string]any)["get"].(map[string]any)
	parameters := operation["parameters"].([]any)
	var sourceType map[string]any
	for _, value := range parameters {
		parameter := value.(map[string]any)
		if parameter["name"] == "source_type" {
			sourceType = parameter
			break
		}
	}
	require.NotNil(t, sourceType)
	require.ElementsMatch(t, []any{"organization", "workspace", "user", "agent"},
		sourceType["schema"].(map[string]any)["enum"].([]any))

	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	shared := schemas["SharedNode"].(map[string]any)
	require.Subset(t, shared["required"].([]any), []any{"sources", "original_path", "source_namespace"})
	properties := shared["properties"].(map[string]any)
	require.Equal(t, "#/components/schemas/SharedGrantSource",
		properties["sources"].(map[string]any)["items"].(map[string]any)["$ref"])
	source := schemas["SharedGrantSource"].(map[string]any)
	require.Subset(t, source["required"].([]any),
		[]any{"grant_id", "principal_type", "principal_id", "expires_at"})
	principalType := source["properties"].(map[string]any)["principal_type"].(map[string]any)
	require.Equal(t, "string", principalType["type"])
	require.NotContains(t, principalType, "enum")
	require.Equal(t, 64, principalType["maxLength"])
	require.Contains(t, principalType["description"], "SERVICE_ACCOUNT")
	writePrincipalType := schemas["CreateTypedGrantRequest"].(map[string]any)["properties"].(map[string]any)["principal_type"].(map[string]any)
	require.ElementsMatch(t,
		[]any{"USER", "WORKSPACE", "ORGANIZATION", "ROLE"},
		writePrincipalType["enum"].([]any),
	)
	sourceNamespace := schemas["SharedNamespaceSource"].(map[string]any)
	require.Subset(t, sourceNamespace["required"].([]any),
		[]any{"organization_id", "workspace_id"})
}

func TestFilesystemAuditOpenAPISchemaMatchesSDKContract(t *testing.T) {
	raw, err := specopenapi.Files.ReadFile("filesystem.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &document))

	paths := document["paths"].(map[string]any)
	operation := paths["/api/v1/workspaces/{wsId}/filesystem-audits"].(map[string]any)["get"].(map[string]any)
	responses := operation["responses"].(map[string]any)
	response := responses["200"].(map[string]any)
	content := response["content"].(map[string]any)
	schema := content["application/json"].(map[string]any)["schema"].(map[string]any)
	allOf := schema["allOf"].([]any)
	override := allOf[1].(map[string]any)
	data := override["properties"].(map[string]any)["data"].(map[string]any)
	require.Equal(t, "#/components/schemas/FileAuditEventPage", data["$ref"])

	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	page := schemas["FileAuditEventPage"].(map[string]any)
	pageProperties := page["properties"].(map[string]any)
	require.Equal(t, "#/components/schemas/FileAuditEvent",
		pageProperties["items"].(map[string]any)["items"].(map[string]any)["$ref"])
	require.Contains(t, pageProperties, "next_cursor")
	event := schemas["FileAuditEvent"].(map[string]any)
	eventProperties := event["properties"].(map[string]any)
	for _, field := range []string{
		"id", "actor_id", "action", "resource_type", "resource_id",
		"result", "trace_id", "created_at", "detail",
	} {
		require.Contains(t, eventProperties, field)
	}
	require.Equal(t, "uuid", eventProperties["id"].(map[string]any)["format"])
	require.Equal(t, "uuid", eventProperties["actor_id"].(map[string]any)["format"])
	require.Equal(t, "object", eventProperties["detail"].(map[string]any)["type"])
	require.ElementsMatch(t,
		[]any{"items", "next_cursor"},
		page["required"].([]any),
	)
	require.Contains(t, event["required"].([]any), "detail")
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
		"POST /api/v1/workspaces/{wsId}/transfers":                                     {"400", "401", "403", "404", "409", "415", "422", "500", "503"},
		"GET /api/v1/workspaces/{wsId}/transfers/{sessionId}":                          {"400", "401", "403", "404", "422", "500", "503"},
		"DELETE /api/v1/workspaces/{wsId}/transfers/{sessionId}":                       {"400", "401", "403", "404", "409", "422", "500", "503"},
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

type openAPIOrganizations struct{}

func (openAPIOrganizations) IsOrgMember(context.Context, kernel.OrgID, kernel.MemberID) (bool, error) {
	return true, nil
}
func (openAPIOrganizations) HasOrgPermission(context.Context, kernel.OrgID, kernel.MemberID, string) (bool, error) {
	return true, nil
}

type openAPIWorkspaceAccess struct{}

func (openAPIWorkspaceAccess) ResolveWorkspaceAccess(context.Context, kernel.WorkspaceID, kernel.MemberID) (kernel.OrgID, bool, error) {
	return kernel.OrgID{}, true, nil
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
	filesystemhttp.RegisterGovernanceRoutes(engine, pass, pass, openAPIOrganizations{}, openAPIWorkspaceAccess{}, &filesystemhttp.GovernanceHandler{})
	filesystemhttp.RegisterRoutes(engine, pass, pass, openAPIMembers{}, openAPIPermissions{}, &filesystemhttp.Handler{})
	actual := map[string]bool{}
	for _, route := range engine.Routes() {
		path := strings.ReplaceAll(route.Path, ":wsId", "{wsId}")
		path = strings.ReplaceAll(path, ":namespaceId", "{namespaceId}")
		path = strings.ReplaceAll(path, ":nodeId", "{nodeId}")
		path = strings.ReplaceAll(path, ":revisionId", "{revisionId}")
		path = strings.ReplaceAll(path, ":sessionId", "{sessionId}")
		path = strings.ReplaceAll(path, ":grantId", "{grantId}")
		path = strings.ReplaceAll(path, ":requestId", "{requestId}")
		path = strings.ReplaceAll(path, ":capabilityId", "{capabilityId}")
		path = strings.ReplaceAll(path, ":exportId", "{exportId}")
		path = strings.ReplaceAll(path, ":providerId", "{providerId}")
		path = strings.ReplaceAll(path, ":poolId", "{poolId}")
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
