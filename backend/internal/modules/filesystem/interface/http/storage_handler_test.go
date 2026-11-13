package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
)

func TestCreateStoragePoolRequiresStorageAdmin(t *testing.T) {
	res := postAs(t, ordinaryMemberPermissions, "/storage-pools", `{"name":"archive"}`)
	assert.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "AUTH_FORBIDDEN")
}

func TestCreateStoragePoolAcceptsStorageAdmin(t *testing.T) {
	var captured command.CreateStoragePool
	orgID := uuid.New()
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: uuid.New(), OrganizationID: orgID, WorkspaceID: workspaceID}, nil
		},
		createStoragePool: func(_ context.Context, cmd command.CreateStoragePool) (command.CreateStoragePoolResult, error) {
			captured = cmd
			return command.CreateStoragePoolResult{ID: uuid.New(), Name: cmd.Name}, nil
		},
	}
	res := postAsWithHandler(t, h, storageAdminPermissions, "/storage-pools", `{"name":"archive"}`)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
	require.Equal(t, orgID, captured.OrganizationID)
	require.Equal(t, "archive", captured.Name)
}

func TestStorageRoutesUseExpectedPermissions(t *testing.T) {
	tests := []struct {
		method, path, permission string
	}{
		{http.MethodGet, "/storage-providers", authz.PermStorageRead},
		{http.MethodPost, "/storage-providers", authz.PermStorageAdmin},
		{http.MethodGet, "/storage-pools", authz.PermStorageRead},
		{http.MethodPost, "/storage-pools", authz.PermStorageAdmin},
		{http.MethodGet, "/storage-bindings", authz.PermStorageRead},
		{http.MethodPost, "/storage-bindings", authz.PermStorageAdmin},
		{http.MethodGet, "/lifecycle-policies", authz.PermStorageRead},
		{http.MethodPost, "/lifecycle-policies", authz.PermLifecycleAdmin},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			h := &Handler{
				namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
					return &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: workspaceID}, nil
				},
			}
			res := requestAs(t, h, memberStub(true), permissionStub{allowed: map[string]bool{tt.permission: true}}, tt.method, tt.path, `{}`)
			require.NotEqual(t, http.StatusForbidden, res.Code, res.Body.String())
		})
	}
}

var (
	ordinaryMemberPermissions = permissionStub{allowed: map[string]bool{
		authz.PermFileRead: true, authz.PermFileWrite: true,
	}}
	storageAdminPermissions = permissionStub{allowed: map[string]bool{
		authz.PermStorageAdmin: true,
	}}
)

func postAs(t *testing.T, permissions permissionStub, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: workspaceID}, nil
		},
	}
	return postAsWithHandler(t, h, permissions, path, body)
}

func postAsWithHandler(t *testing.T, h *Handler, permissions permissionStub, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return requestAs(t, h, memberStub(true), permissions, http.MethodPost, path, body)
}

func requestAs(t *testing.T, h *Handler, members authz.WorkspaceMemberChecker, permissions authz.PermissionChecker, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	router, token, wsID := testRouter(t, members, permissions, h)
	req := httptest.NewRequest(method, "/api/v1/workspaces/"+wsID.String()+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if method != http.MethodGet && method != http.MethodDelete {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "storage-test-key")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}
