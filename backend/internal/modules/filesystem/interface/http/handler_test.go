package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type permissionStub struct {
	allowed map[string]bool
}

func (s permissionStub) RequirePermission(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, permission string) error {
	if s.allowed[permission] {
		return nil
	}
	return kernel.NewDomainError("AUTH_FORBIDDEN", "forbidden")
}

func (s permissionStub) HasPermission(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, permission string) (bool, error) {
	return s.allowed[permission], nil
}

type memberStub bool

func (s memberStub) IsWorkspaceMember(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return bool(s), nil
}

type auditStub struct{ entries []audit.Entry }

func (s *auditStub) Log(_ context.Context, entry audit.Entry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func TestCreateDirectoryRequiresMembershipPermissionAndIdempotencyKey(t *testing.T) {
	tests := []struct {
		name        string
		member      bool
		permissions map[string]bool
		key         string
		wantStatus  int
		wantCode    string
	}{
		{name: "not a member", permissions: map[string]bool{authz.PermFileWrite: true}, key: "request-1", wantStatus: http.StatusForbidden, wantCode: "TENANT_NOT_WORKSPACE_MEMBER"},
		{name: "permission denied", member: true, permissions: map[string]bool{}, key: "request-1", wantStatus: http.StatusForbidden, wantCode: "AUTH_FORBIDDEN"},
		{name: "missing key", member: true, permissions: map[string]bool{authz.PermFileWrite: true}, wantStatus: http.StatusBadRequest, wantCode: "FILESYSTEM_IDEMPOTENCY_KEY_REQUIRED"},
		{name: "invalid key", member: true, permissions: map[string]bool{authz.PermFileWrite: true}, key: strings.Repeat("x", 129), wantStatus: http.StatusBadRequest, wantCode: "FILESYSTEM_IDEMPOTENCY_KEY_INVALID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, token, wsID := testRouter(t, memberStub(tt.member), permissionStub{allowed: tt.permissions}, &Handler{})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/directories", strings.NewReader(`{"namespace_id":"`+uuid.NewString()+`","name":"docs"}`))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			if tt.key != "" {
				req.Header.Set("Idempotency-Key", tt.key)
			}
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			require.Equal(t, tt.wantStatus, res.Code, res.Body.String())
			require.Contains(t, res.Body.String(), tt.wantCode)
		})
	}
}

func TestListFilesUsesReadPermissionAndDoesNotRequireIdempotencyKey(t *testing.T) {
	nsID := uuid.New()
	rootID := uuid.New()
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, &Handler{
		listChildren: func(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error) {
			return []model.Node{}, "", nil
		},
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, WorkspaceID: workspaceID, RootNodeID: &rootID}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files?limit=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
}

func TestListFilesDeniesWhenResourceAuthorizerDeniesAfterRBAC(t *testing.T) {
	nsID, rootID := uuid.New(), uuid.New()
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, OrganizationID: uuid.New(), WorkspaceID: workspaceID, RootNodeID: &rootID}, nil
		},
		authorizeResource: func(_ context.Context, request resourceauth.Request) resourceauth.Decision {
			require.Equal(t, rootID, request.NodeID)
			require.Equal(t, model.ResourcePermissionRead, request.Permission)
			return resourceauth.Decision{Reason: "default_deny"}
		},
		listChildren: func(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error) {
			t.Fatal("resource denial must stop query")
			return nil, "", nil
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "FILESYSTEM_RESOURCE_FORBIDDEN")
}

func TestListFilesFiltersDeniedChildrenAndFillsAcrossPages(t *testing.T) {
	nsID, rootID := uuid.New(), uuid.New()
	ids := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		uuid.MustParse("00000000-0000-0000-0000-000000000004"),
		uuid.MustParse("00000000-0000-0000-0000-000000000005"),
	}
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, WorkspaceID: workspaceID, RootNodeID: &rootID}, nil
		},
		listChildren: func(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ int, cursor string) ([]model.Node, string, error) {
			if cursor == "" {
				return []model.Node{{ID: ids[0], Name: "deny-1"}, {ID: ids[1], Name: "allow-2"}, {ID: ids[2], Name: "deny-3"}}, ids[2].String(), nil
			}
			return []model.Node{{ID: ids[3], Name: "allow-4"}, {ID: ids[4], Name: "deny-5"}}, "", nil
		},
		authorizeResource: func(_ context.Context, request resourceauth.Request) resourceauth.Decision {
			if request.NodeID == rootID || request.NodeID == ids[1] || request.NodeID == ids[3] {
				return resourceauth.Decision{Allowed: true}
			}
			return resourceauth.Decision{Reason: "explicit_deny"}
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files?limit=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "allow-2")
	require.Contains(t, res.Body.String(), "allow-4")
	require.NotContains(t, res.Body.String(), "deny-")
	require.Contains(t, res.Body.String(), ids[3].String())
}

func TestListFilesReturnsEmptyWhenAllChildrenDenied(t *testing.T) {
	nsID, rootID, deniedID := uuid.New(), uuid.New(), uuid.New()
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, WorkspaceID: workspaceID, RootNodeID: &rootID}, nil
		},
		listChildren: func(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error) {
			return []model.Node{{ID: deniedID, Name: "hidden"}}, "", nil
		},
		authorizeResource: func(_ context.Context, request resourceauth.Request) resourceauth.Decision {
			if request.NodeID == rootID {
				return resourceauth.Decision{Allowed: true}
			}
			return resourceauth.Decision{Reason: "explicit_deny"}
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files?limit=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	require.NotContains(t, res.Body.String(), "hidden")
	require.Contains(t, res.Body.String(), `"items":[]`)
}

func TestCreateDirectoryRejectsUnknownIdentityAndOversizedBodies(t *testing.T) {
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileWrite: true}}, &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: uuid.New(), WorkspaceID: workspaceID}, nil
		},
	})
	cases := []string{
		`{"namespace_id":"` + uuid.NewString() + `","name":"docs","actor_id":"` + uuid.NewString() + `"}`,
		`{"namespace_id":"` + uuid.NewString() + `","name":"` + strings.Repeat("x", maxJSONBodyBytes) + `"}`,
	}
	for _, body := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/directories", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "request-1")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
		require.Contains(t, res.Body.String(), "FILESYSTEM_INVALID_REQUEST")
	}
}

func TestMutationCommandIDIsDeterministicAndWorkspaceScoped(t *testing.T) {
	ws1, ws2 := uuid.New(), uuid.New()
	first := commandID(ws1, "client-key")
	require.Equal(t, first, commandID(ws1, "client-key"))
	require.NotEqual(t, first, commandID(ws2, "client-key"))
	require.NotEqual(t, first, commandID(ws1, "other-key"))
	require.NotEqual(t, uuid.Nil, first)
}

func TestCreateDirectoryReturnsCreatedAndContextIdentity(t *testing.T) {
	nsID := uuid.New()
	rootID := uuid.New()
	var captured command.CreateDirectory
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, OrganizationID: uuid.New(), WorkspaceID: workspaceID, RootNodeID: &rootID}, nil
		},
		createDirectory: func(_ context.Context, cmd command.CreateDirectory) (*model.Node, error) {
			captured = cmd
			return &model.Node{ID: uuid.New(), NamespaceID: cmd.NamespaceID, Name: cmd.Name, Type: model.NodeTypeDirectory, State: model.NodeStateActive, OwnerID: cmd.ActorID}, nil
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileWrite: true}}, h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/directories", strings.NewReader(`{"namespace_id":"`+nsID.String()+`","name":"docs"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Idempotency-Key", "create-docs")
	req.Header.Set("X-Trace-Id", "trace-123")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
	require.Equal(t, wsID.UUID, captured.WorkspaceID)
	require.Equal(t, nsID, captured.NamespaceID)
	require.Equal(t, "trace-123", captured.TraceID)
	require.NotEqual(t, uuid.Nil, captured.ActorID)
}

func TestCreateDirectoryValidatesBodyNamespaceID(t *testing.T) {
	namespaceID := uuid.New()
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: namespaceID, OrganizationID: uuid.New(), WorkspaceID: workspaceID}, nil
		},
		createDirectory: func(context.Context, command.CreateDirectory) (*model.Node, error) {
			t.Fatal("invalid namespace_id must not reach command handler")
			return nil, nil
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileWrite: true}}, h)
	tests := []struct {
		name       string
		namespace  string
		wantStatus int
		wantCode   string
	}{
		{name: "uppercase", namespace: strings.ToUpper(namespaceID.String()), wantStatus: http.StatusBadRequest, wantCode: "FILESYSTEM_INVALID_REQUEST"},
		{name: "nil", namespace: uuid.Nil.String(), wantStatus: http.StatusBadRequest, wantCode: "FILESYSTEM_INVALID_REQUEST"},
		{name: "malformed", namespace: "not-a-uuid", wantStatus: http.StatusBadRequest, wantCode: "FILESYSTEM_INVALID_REQUEST"},
		{name: "mismatch", namespace: uuid.NewString(), wantStatus: http.StatusNotFound, wantCode: "FILESYSTEM_NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"namespace_id":"` + tt.namespace + `","name":"docs"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/directories", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "namespace-"+tt.name)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			require.Equal(t, tt.wantStatus, res.Code, res.Body.String())
			require.Contains(t, res.Body.String(), tt.wantCode)
		})
	}
}

func TestRoutesUseExpectedPermissions(t *testing.T) {
	nodeID, revisionID := uuid.NewString(), uuid.NewString()
	tests := []struct {
		method, path, permission string
	}{
		{http.MethodGet, "/namespaces", authz.PermFileRead},
		{http.MethodGet, "/files", authz.PermFileRead},
		{http.MethodPost, "/directories", authz.PermFileWrite},
		{http.MethodPatch, "/files/" + nodeID + "/name", authz.PermFileWrite},
		{http.MethodPost, "/files/" + nodeID + "/move", authz.PermFileWrite},
		{http.MethodPost, "/files/" + nodeID + "/copy", authz.PermFileWrite},
		{http.MethodDelete, "/files/" + nodeID, authz.PermFileDelete},
		{http.MethodGet, "/files/" + nodeID + "/revisions", authz.PermFileRead},
		{http.MethodPost, "/files/" + nodeID + "/revisions/" + revisionID + "/restore", authz.PermFileWrite},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{tt.permission: true}}, &Handler{})
			req := httptest.NewRequest(tt.method, "/api/v1/workspaces/"+wsID.String()+tt.path, strings.NewReader(`{}`))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "route-test")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			require.NotEqual(t, http.StatusForbidden, res.Code, res.Body.String())
		})
	}
}

func TestMiddlewareShortCircuitOrder(t *testing.T) {
	router := httpx.NewEngine()
	tokens := authn.NewTokenProvider(config.JWTConfig{})
	entitlementCalls := 0
	RegisterRoutes(router, func(c *gin.Context) {
		entitlementCalls++
		httpx.Fail(c, "ENTITLEMENT_MODULE_DISABLED", "disabled", http.StatusForbidden)
		c.Abort()
	}, authn.RequireAuth(tokens), memberStub(true), permissionStub{allowed: map[string]bool{}}, &Handler{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+uuid.NewString()+"/files", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusUnauthorized, res.Code)
	require.Zero(t, entitlementCalls)

	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/not-a-uuid/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "TENANT_WORKSPACE_INVALID")
	require.Zero(t, entitlementCalls)

	nonMember := httpx.NewEngine()
	RegisterRoutes(nonMember, func(c *gin.Context) {
		entitlementCalls++
		c.Next()
	}, authn.RequireAuth(tokens), memberStub(false), permissionStub{allowed: map[string]bool{}}, &Handler{})
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+uuid.NewString()+"/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res = httptest.NewRecorder()
	nonMember.ServeHTTP(res, req)
	require.Equal(t, http.StatusForbidden, res.Code)
	require.Zero(t, entitlementCalls)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+uuid.NewString()+"/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusForbidden, res.Code)
	require.Contains(t, res.Body.String(), "ENTITLEMENT_MODULE_DISABLED")
	require.Equal(t, 1, entitlementCalls)
}

func TestRBACMiddlewareDenialAuditIncludesOrganization(t *testing.T) {
	organizationID := uuid.New()
	logs := &auditStub{}
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: uuid.New(), OrganizationID: organizationID, WorkspaceID: workspaceID}, nil
		},
		audit: logs,
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{}}, h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusForbidden, res.Code)
	require.Len(t, logs.entries, 1)
	require.NotNil(t, logs.entries[0].OrgID)
	require.Equal(t, organizationID, logs.entries[0].OrgID.UUID)
}

func TestListFilesDefaultsToNamespaceRootAndNeverReturnsHiddenRoot(t *testing.T) {
	rootID, childID, nsID := uuid.New(), uuid.New(), uuid.New()
	var capturedParent *uuid.UUID
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, WorkspaceID: workspaceID, RootNodeID: &rootID}, nil
		},
		getNode: func(_ context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
			require.Equal(t, nsID, namespaceID)
			require.Equal(t, rootID, nodeID)
			return &model.Node{ID: rootID, NamespaceID: nsID, Type: model.NodeTypeDirectory}, nil
		},
		listChildren: func(_ context.Context, namespaceID uuid.UUID, parentID *uuid.UUID, _ int, _ string) ([]model.Node, string, error) {
			require.Equal(t, nsID, namespaceID)
			capturedParent = parentID
			return []model.Node{{ID: childID, NamespaceID: nsID, ParentID: &rootID, Name: "docs", Type: model.NodeTypeDirectory}}, "", nil
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.NotNil(t, capturedParent)
	require.Equal(t, rootID, *capturedParent)
	require.NotContains(t, res.Body.String(), ".root")
	require.Contains(t, res.Body.String(), "docs")
}

func TestListFilesRejectsForeignExplicitParent(t *testing.T) {
	rootID, nsID, foreignNS, parentID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, WorkspaceID: workspaceID, RootNodeID: &rootID}, nil
		},
		getNode: func(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: parentID, NamespaceID: foreignNS, Type: model.NodeTypeDirectory}, nil
		},
		listChildren: func(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error) {
			t.Fatal("list must not run for a foreign parent")
			return nil, "", nil
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files?parent_id="+parentID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
}

func TestMutationJSONAndEmptyBodyContracts(t *testing.T) {
	nsID := uuid.New()
	h := &Handler{namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
		return &model.Namespace{ID: nsID, WorkspaceID: workspaceID}, nil
	}}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{
		authz.PermFileWrite: true, authz.PermFileDelete: true,
	}}, h)
	for _, contentType := range []string{"", "text/plain", "application/jsonp"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/directories", strings.NewReader(`{"name":"docs"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Idempotency-Key", "key")
		req.Header.Set("Content-Type", contentType)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusUnsupportedMediaType, res.Code, res.Body.String())
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/"+wsID.String()+"/files/"+uuid.NewString(), io.NopCloser(strings.NewReader(`{}`)))
	req.ContentLength = -1
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", "key")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
}

func TestTrashHTTPReplayReturnsNoContentAndConflictsOnSameKeyDifferentTarget(t *testing.T) {
	nsID, rootID, firstNode, secondNode := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	claims := map[uuid.UUID]uuid.UUID{}
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, OrganizationID: uuid.New(), WorkspaceID: workspaceID, RootNodeID: &rootID}, nil
		},
		authorizeResource: func(context.Context, resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Allowed: true, Reason: "owner"}
		},
		trashNode: func(_ context.Context, cmd command.TrashNode) error {
			if !cmd.Allowed {
				return domain.ErrResourceDenied
			}
			if claimed, ok := claims[cmd.CommandID]; ok && claimed != cmd.NodeID {
				return domain.ErrIdempotencyConflict
			}
			claims[cmd.CommandID] = cmd.NodeID
			return nil
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileDelete: true}}, h)
	send := func(nodeID uuid.UUID, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/"+wsID.String()+"/files/"+nodeID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Idempotency-Key", key)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	require.Equal(t, http.StatusNoContent, send(firstNode, "same").Code)
	require.Equal(t, http.StatusNoContent, send(firstNode, "same").Code)
	require.Equal(t, http.StatusConflict, send(secondNode, "same").Code)

	h.authorizeResource = func(context.Context, resourceauth.Request) resourceauth.Decision {
		return resourceauth.Decision{Reason: "default_deny"}
	}
	require.Equal(t, http.StatusForbidden, send(firstNode, "other").Code)
}

func TestUUIDsMustBeCanonicalNonNil(t *testing.T) {
	nsID := uuid.New()
	h := &Handler{namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
		return &model.Namespace{ID: nsID, WorkspaceID: workspaceID}, nil
	}}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, h)
	for _, nodeID := range []string{strings.ToUpper(uuid.NewString()), strings.ReplaceAll(uuid.NewString(), "-", ""), uuid.Nil.String()} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files/"+nodeID+"/revisions", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestStableFilesystemErrorMapping(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrUnsupportedNodeType, http.StatusUnprocessableEntity, "FILESYSTEM_UNSUPPORTED_OPERATION"},
		{domain.ErrRootMutation, http.StatusConflict, "FILESYSTEM_CONFLICT"},
		{domain.ErrNodeIDConflict, http.StatusConflict, "FILESYSTEM_CONFLICT"},
		{domain.ErrCommandInProgress, http.StatusConflict, "FILESYSTEM_CONFLICT"},
		{domain.ErrQuotaExceeded, http.StatusUnprocessableEntity, "FILESYSTEM_QUOTA_EXCEEDED"},
		{storage.ErrOutcomeUnknown, http.StatusServiceUnavailable, "FILESYSTEM_OUTCOME_UNKNOWN"},
		{domain.ErrTransactionRetry, http.StatusServiceUnavailable, "FILESYSTEM_RETRY_EXHAUSTED"},
		{domain.ErrTransactionExhausted, http.StatusServiceUnavailable, "FILESYSTEM_RETRY_EXHAUSTED"},
		{domain.ErrPersistence, http.StatusServiceUnavailable, "FILESYSTEM_UNAVAILABLE"},
		{errors.New("provider object-key secret"), http.StatusInternalServerError, "FILESYSTEM_INTERNAL_ERROR"},
	}
	for _, tt := range tests {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		filesystemError(c, tt.err)
		require.Equal(t, tt.status, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), tt.code)
		require.NotContains(t, recorder.Body.String(), "object-key secret")
	}
}

func TestCrossWorkspaceNamespaceIsNotVisible(t *testing.T) {
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, &Handler{
		namespaceForWorkspace: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: uuid.New(), WorkspaceID: uuid.New()}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/namespaces", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
	require.NotContains(t, res.Body.String(), "namespace mismatch")
}

func TestListFilesValidatesLimitAndCursor(t *testing.T) {
	h := &Handler{namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
		return &model.Namespace{ID: uuid.New(), WorkspaceID: workspaceID}, nil
	}}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, h)
	for _, query := range []string{"?limit=0", "?limit=201", "?cursor=not-a-uuid"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
		require.Contains(t, res.Body.String(), "FILESYSTEM_INVALID_REQUEST")
	}
}

func testRouter(t *testing.T, members authz.WorkspaceMemberChecker, permissions authz.PermissionChecker, h *Handler) (*gin.Engine, string, kernel.WorkspaceID) {
	t.Helper()
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "filesystem-test"})
	memberID := kernel.NewMemberID()
	token, err := tokens.Issue(memberID)
	require.NoError(t, err)
	wsID := kernel.NewWorkspaceID()
	router := httpx.NewEngine()
	if h.authorizeResource == nil {
		h.authorizeResource = func(context.Context, resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Allowed: true, Reason: "test"}
		}
	}
	RegisterRoutes(router, nil, authn.RequireAuth(tokens), members, permissions, h)
	return router, token, wsID
}
