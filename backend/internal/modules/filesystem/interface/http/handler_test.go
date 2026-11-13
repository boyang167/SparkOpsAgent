package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
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
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, &Handler{
		listChildren: func(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error) {
			return []model.Node{}, "", nil
		},
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, WorkspaceID: workspaceID}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/files?limit=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
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
	var captured command.CreateDirectory
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: nsID, OrganizationID: uuid.New(), WorkspaceID: workspaceID}, nil
		},
		createDirectory: func(_ context.Context, cmd command.CreateDirectory) (*model.Node, error) {
			captured = cmd
			return &model.Node{ID: uuid.New(), NamespaceID: cmd.NamespaceID, Name: cmd.Name, Type: model.NodeTypeDirectory, State: model.NodeStateActive, OwnerID: cmd.ActorID}, nil
		},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileWrite: true}}, h)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/directories", strings.NewReader(`{"namespace_id":"`+nsID.String()+`","name":"docs"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
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

func TestEntitlementRunsBeforeAuthentication(t *testing.T) {
	router := httpx.NewEngine()
	tokens := authn.NewTokenProvider(config.JWTConfig{})
	RegisterRoutes(router, func(c *gin.Context) {
		httpx.Fail(c, "ENTITLEMENT_MODULE_DISABLED", "disabled", http.StatusForbidden)
		c.Abort()
	}, authn.RequireAuth(tokens), memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileRead: true}}, &Handler{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+uuid.NewString()+"/files", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusForbidden, res.Code)
	require.Contains(t, res.Body.String(), "ENTITLEMENT_MODULE_DISABLED")
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
	RegisterRoutes(router, nil, authn.RequireAuth(tokens), members, permissions, h)
	return router, token, wsID
}
