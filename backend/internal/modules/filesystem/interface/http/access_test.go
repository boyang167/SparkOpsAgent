package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	filesystemport "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
)

type allowGrantPort struct{}

func (allowGrantPort) Create(context.Context, filesystemport.GrantRequest) (model.ResourceGrant, error) {
	return model.ResourceGrant{ID: uuid.New()}, nil
}
func (allowGrantPort) Revoke(context.Context, filesystemport.GrantRequest, uuid.UUID) error {
	return nil
}
func (allowGrantPort) List(context.Context, filesystemport.GrantRequest) ([]model.ResourceGrant, error) {
	return nil, nil
}

func TestGrantEndpointRejectsCrossOrganization(t *testing.T) {
	org := uuid.New()
	namespaceID := uuid.New()
	nodeID := uuid.New()
	grants := command.NewGrantResourceHandler(allowGrantPort{})
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: namespaceID, OrganizationID: org, WorkspaceID: workspaceID, Status: "ACTIVE"}, nil
		},
		access: ResourceAccess{Grant: grants.Handle},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileShare: true}}, h)
	body := `{"grantee_id":"` + uuid.NewString() + `","grantee_organization_id":"` + uuid.NewString() + `","permission":"READ","effect":"ALLOW"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/files/"+nodeID.String()+"/grants", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "grant-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "FILESYSTEM_CROSS_ORGANIZATION_GRANT")
}
