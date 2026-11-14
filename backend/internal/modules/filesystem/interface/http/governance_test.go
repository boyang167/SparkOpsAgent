package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type governanceOrgStub struct {
	active bool
}

func (s governanceOrgStub) IsOrgMember(context.Context, kernel.OrgID, kernel.MemberID) (bool, error) {
	return s.active, nil
}
func (s governanceOrgStub) HasOrgPermission(context.Context, kernel.OrgID, kernel.MemberID, string) (bool, error) {
	return false, nil
}

type governanceWorkspaceStub struct {
	orgID  uuid.UUID
	active bool
}

func (s governanceWorkspaceStub) ResolveWorkspaceAccess(context.Context, kernel.WorkspaceID, kernel.MemberID) (kernel.OrgID, bool, error) {
	return kernel.OrgID{UUID: s.orgID}, s.active, nil
}

func TestGovernanceMissingOrMalformedOrganizationHeaderReturnsBadRequest(t *testing.T) {
	router, token, _, _ := governanceRouter(t, governanceOrgStub{active: true}, governanceWorkspaceStub{}, &GovernanceHandler{})
	for _, header := range []string{"", "NOT-A-UUID"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/namespaces", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		if header != "" {
			req.Header.Set("X-Organization-ID", header)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
	}
}

func TestGovernanceInvalidWorkspaceAudienceReturnsForbidden(t *testing.T) {
	router, token, orgID, _ := governanceRouter(t, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgIDOrNew(uuid.Nil), active: false}, &GovernanceHandler{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/namespaces", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	req.Header.Set("X-Workspace-ID", uuid.NewString())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
}

func TestGovernanceChildrenConcealsUnauthorizedNamespaceAndParent(t *testing.T) {
	orgID, workspaceID := uuid.New(), uuid.New()
	namespaceID, rootID := uuid.New(), uuid.New()
	handler := &GovernanceHandler{
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: orgID, WorkspaceID: workspaceID, RootNodeID: &rootID,
			}, nil
		},
		authorize: func(context.Context, resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Reason: "default_deny"}
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgID, active: true}, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/namespaces/"+namespaceID.String()+"/children", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	req.Header.Set("X-Workspace-ID", workspaceID.String())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "FILESYSTEM_NOT_FOUND")
}

func TestGovernanceSharedWithMeForwardsQueryCursorAndLimit(t *testing.T) {
	orgID, workspaceID := uuid.New(), uuid.New()
	cursor := uuid.NewString()
	var captured query.SharedWithMe
	handler := &GovernanceHandler{
		sharedWithMe: func(_ context.Context, input query.SharedWithMe) ([]repository.SharedNode, string, error) {
			captured = input
			return []repository.SharedNode{}, "", nil
		},
	}
	router, token, _, memberID := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgID, active: true}, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/shared-with-me?q=budget&cursor="+cursor+"&limit=23", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	req.Header.Set("X-Workspace-ID", workspaceID.String())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Equal(t, memberID.UUID, captured.ActorID)
	require.Equal(t, orgID, captured.OrganizationID)
	require.Equal(t, workspaceID, captured.WorkspaceID)
	require.Equal(t, "budget", captured.Query)
	require.Equal(t, cursor, captured.Cursor)
	require.Equal(t, 23, captured.Limit)
}

func governanceRouter(t *testing.T, orgs authz.OrgPermissionChecker, workspaces authz.WorkspaceAccessResolver, handler *GovernanceHandler) (*gin.Engine, string, uuid.UUID, kernel.MemberID) {
	return governanceRouterWithIDs(t, uuid.New(), orgs, workspaces, handler)
}

func governanceRouterWithIDs(t *testing.T, orgID uuid.UUID, orgs authz.OrgPermissionChecker, workspaces authz.WorkspaceAccessResolver, handler *GovernanceHandler) (*gin.Engine, string, uuid.UUID, kernel.MemberID) {
	t.Helper()
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "filesystem-governance-test"})
	memberID := kernel.NewMemberID()
	token, err := tokens.Issue(memberID)
	require.NoError(t, err)
	router := httpx.NewEngine()
	RegisterGovernanceRoutes(router, nil, authn.RequireAuth(tokens), orgs, workspaces, handler)
	return router, token, orgID, memberID
}

func orgIDOrNew(id uuid.UUID) uuid.UUID {
	if id == uuid.Nil {
		return uuid.New()
	}
	return id
}
