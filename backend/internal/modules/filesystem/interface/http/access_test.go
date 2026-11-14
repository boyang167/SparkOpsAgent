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

func TestLegacyGrantEndpointIgnoresClientPrincipalOrganizationAndNormalizesUser(t *testing.T) {
	org := uuid.New()
	namespaceID := uuid.New()
	nodeID := uuid.New()
	var captured command.SubmitShare
	h := &Handler{
		namespaceForWorkspace: func(_ context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: namespaceID, OrganizationID: org, WorkspaceID: workspaceID, Status: "ACTIVE"}, nil
		},
		access: ResourceAccess{Share: func(_ context.Context, cmd command.SubmitShare) (command.ShareResult, error) {
			captured = cmd
			grant := model.ResourceGrant{
				ID: uuid.New(), NodeID: cmd.NodeID, OrganizationID: cmd.OrganizationID,
				PrincipalType: cmd.PrincipalType, PrincipalID: cmd.PrincipalIDs[0], GranteeID: cmd.PrincipalIDs[0],
				Permission: cmd.Actions[0], Effect: model.GrantEffectAllow,
			}
			return command.ShareResult{Request: model.ShareRequest{Status: model.ShareRequestApproved}, Grants: []model.ResourceGrant{grant}}, nil
		}},
	}
	router, token, wsID := testRouter(t, memberStub(true), permissionStub{allowed: map[string]bool{authz.PermFileShare: true}}, h)
	body := `{"grantee_id":"` + uuid.NewString() + `","grantee_organization_id":"` + uuid.NewString() + `","permission":"READ","effect":"ALLOW"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/files/"+nodeID.String()+"/grants", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "grant-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
	require.Equal(t, org, captured.OrganizationID)
	require.Equal(t, model.PrincipalUser, captured.PrincipalType)
	require.Equal(t, []model.ResourcePermission{model.ResourcePermissionRead}, captured.Actions)
	require.Len(t, captured.PrincipalIDs, 1)
}

func TestTypedGrantBodyPreservesDownloadAsDistinctPermission(t *testing.T) {
	org, workspaceID := uuid.New(), uuid.New()
	rc := requestContext{
		workspaceID: workspaceID, actorID: uuid.New(), commandID: uuid.New(),
		namespace: &model.Namespace{ID: uuid.New(), OrganizationID: org, WorkspaceID: workspaceID},
	}
	principalID := uuid.New()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	submit, legacy, ok := shareCommand(c, rc, uuid.New(), grantBody{
		PrincipalType: string(model.PrincipalWorkspace), PrincipalIDs: []string{principalID.String()},
		Actions: []string{string(model.ResourcePermissionRead), string(model.ResourcePermissionDownload)},
	})
	require.True(t, ok)
	require.False(t, legacy)
	require.Equal(t, []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionDownload}, submit.Actions)
}

func TestGrantViewIncludesConsistentTypedPrincipalFields(t *testing.T) {
	principalID := uuid.New()
	view := grantView(model.ResourceGrant{
		ID: uuid.New(), NodeID: uuid.New(), GranteeID: principalID,
		PrincipalType: model.PrincipalRole, PrincipalID: principalID,
		Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
	})
	require.Equal(t, model.PrincipalRole, view["principal_type"])
	require.Equal(t, principalID, view["principal_id"])
	require.Equal(t, principalID, view["grantee_id"])
}
