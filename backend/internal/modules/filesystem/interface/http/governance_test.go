package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
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
	authorizerCalled := false
	handler := &GovernanceHandler{
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: orgID, WorkspaceID: workspaceID, RootNodeID: &rootID,
			}, nil
		},
		getNode: func(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: rootID, NamespaceID: namespaceID, Type: model.NodeTypeDirectory, State: model.NodeStateActive}, nil
		},
		listChildren: func(context.Context, uuid.UUID, *uuid.UUID, *model.NodeType, string, int, string) ([]model.Node, string, error) {
			return nil, "", nil
		},
		authorize: func(context.Context, resourceauth.Request) resourceauth.Decision {
			authorizerCalled = true
			return resourceauth.Decision{Reason: "default_deny"}
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgID, active: true}, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/namespaces/"+namespaceID.String()+"/children?parent_id="+rootID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	req.Header.Set("X-Workspace-ID", workspaceID.String())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "FILESYSTEM_NOT_FOUND")
	require.True(t, authorizerCalled)
}

func TestGovernanceChildrenUsesHiddenRootInternallyWhenParentIsOmitted(t *testing.T) {
	orgID, workspaceID := uuid.New(), uuid.New()
	namespaceID, rootID, visibleChild := uuid.New(), uuid.New(), uuid.New()
	handler := &GovernanceHandler{
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: orgID, WorkspaceID: workspaceID, RootNodeID: &rootID,
			}, nil
		},
		getNode: func(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: rootID, NamespaceID: namespaceID, Type: model.NodeTypeDirectory, State: model.NodeStateActive}, nil
		},
		listChildren: func(context.Context, uuid.UUID, *uuid.UUID, *model.NodeType, string, int, string) ([]model.Node, string, error) {
			return []model.Node{{
				ID: visibleChild, NamespaceID: namespaceID, ParentID: &rootID,
				Type: model.NodeTypeDirectory, State: model.NodeStateActive,
			}}, "", nil
		},
		authorize: func(_ context.Context, request resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Allowed: request.NodeID == visibleChild}
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgID, active: true}, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/namespaces/"+namespaceID.String()+"/children", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	req.Header.Set("X-Workspace-ID", workspaceID.String())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), visibleChild.String())
	require.NotContains(t, res.Body.String(), rootID.String())
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

func TestGovernanceChildrenPaginatesAfterAuthorizationWithoutLeakingHiddenCursor(t *testing.T) {
	orgID, workspaceID := uuid.New(), uuid.New()
	namespaceID, rootID := uuid.New(), uuid.New()
	visible1 := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	hidden := uuid.MustParse("20000000-0000-0000-0000-000000000002")
	visible2 := uuid.MustParse("30000000-0000-0000-0000-000000000003")
	visible3 := uuid.MustParse("40000000-0000-0000-0000-000000000004")
	nodes := []model.Node{
		{ID: visible1, NamespaceID: namespaceID, Type: model.NodeTypeFile, State: model.NodeStateActive},
		{ID: hidden, NamespaceID: namespaceID, Type: model.NodeTypeFile, State: model.NodeStateActive},
		{ID: visible2, NamespaceID: namespaceID, Type: model.NodeTypeFile, State: model.NodeStateActive},
		{ID: visible3, NamespaceID: namespaceID, Type: model.NodeTypeFile, State: model.NodeStateActive},
	}
	handler := &GovernanceHandler{
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: orgID, WorkspaceID: workspaceID, RootNodeID: &rootID,
			}, nil
		},
		getNode: func(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: rootID, NamespaceID: namespaceID, Type: model.NodeTypeDirectory, State: model.NodeStateActive}, nil
		},
		listChildren: func(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ *model.NodeType, _ string, _ int, cursor string) ([]model.Node, string, error) {
			if cursor == visible2.String() {
				return nodes[3:], "", nil
			}
			return nodes, "", nil
		},
		authorize: func(_ context.Context, request resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Allowed: request.NodeID != hidden}
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgID, active: true}, handler)
	request := func(cursor string) map[string]any {
		url := "/api/v1/filesystem/namespaces/" + namespaceID.String() + "/children?limit=2"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", workspaceID.String())
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &envelope))
		return envelope["data"].(map[string]any)
	}

	first := request("")
	require.Len(t, first["items"], 2)
	require.Equal(t, visible2.String(), first["next_cursor"])
	require.NotEqual(t, hidden.String(), first["next_cursor"])
	second := request(visible2.String())
	require.Len(t, second["items"], 1)
	require.Equal(t, "", second["next_cursor"])
}

func TestGovernanceRepositoryFailureReturnsInternalError(t *testing.T) {
	for _, repositoryErr := range []error{domain.ErrPersistence, errors.New("database unavailable")} {
		t.Run(repositoryErr.Error(), func(t *testing.T) {
			orgID := uuid.New()
			handler := &GovernanceHandler{
				namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
					return nil, repositoryErr
				},
			}
			router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{}, handler)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/namespaces/"+uuid.NewString()+"/children", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Organization-ID", orgID.String())
			res := httptest.NewRecorder()

			router.ServeHTTP(res, req)

			require.Equal(t, http.StatusInternalServerError, res.Code, res.Body.String())
			require.Contains(t, res.Body.String(), "FILESYSTEM_INTERNAL_ERROR")
		})
	}
}

func TestGovernanceNamespaceBudgetExceededReturnsServiceUnavailable(t *testing.T) {
	orgID := uuid.New()
	handler := &GovernanceHandler{
		listNamespaces: func(context.Context, query.ListNamespaces) ([]query.NamespaceSummary, error) {
			return nil, domain.ErrAuthorizationScanBudgetExceeded
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{}, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/namespaces", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusServiceUnavailable, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "FILESYSTEM_AUTHORIZATION_SCAN_BUDGET_EXCEEDED")
}

func TestGovernanceInvalidEncryptedCursorReturnsBadRequest(t *testing.T) {
	orgID := uuid.New()
	handler := &GovernanceHandler{
		sharedWithMe: func(context.Context, query.SharedWithMe) ([]repository.SharedNode, string, error) {
			return nil, "", domain.ErrInvalidGovernanceCursor
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{}, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/shared-with-me?cursor=tampered", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "FILESYSTEM_INVALID_REQUEST")
}

func TestGovernedShareHTTPRequiresIdempotencyAndForwardsTypedDownload(t *testing.T) {
	orgID, workspaceID, namespaceID, nodeID, principalID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	var captured command.SubmitShare
	handler := &GovernanceHandler{
		nodeByID: func(context.Context, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: nodeID, NamespaceID: namespaceID}, nil
		},
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: orgID, WorkspaceID: workspaceID}, nil
		},
		submitShare: func(_ context.Context, cmd command.SubmitShare) (command.ShareResult, error) {
			captured = cmd
			return command.ShareResult{Request: model.ShareRequest{
				ID: uuid.New(), Status: model.ShareRequestPending, PrincipalType: cmd.PrincipalType,
				PrincipalIDs: cmd.PrincipalIDs, Actions: cmd.Actions,
			}}, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgID, active: true}, handler)
	body := `{"principal_type":"WORKSPACE","principal_ids":["` + principalID.String() + `"],"actions":["READ","DOWNLOAD"]}`
	send := func(key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/filesystem/nodes/"+nodeID.String()+"/share-requests", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", workspaceID.String())
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	require.Equal(t, http.StatusBadRequest, send("").Code)
	response := send("share-1")
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.Equal(t, model.PrincipalWorkspace, captured.PrincipalType)
	require.Equal(t, []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionDownload}, captured.Actions)
}

func TestApproveShareRequestHTTPForwardsAuthoritativeOrganization(t *testing.T) {
	orgID, requestID := uuid.New(), uuid.New()
	var captured command.ReviewShareRequest
	handler := &GovernanceHandler{
		approveShare: func(_ context.Context, cmd command.ReviewShareRequest) (model.ShareRequest, error) {
			captured = cmd
			return model.ShareRequest{ID: cmd.RequestID, OrganizationID: cmd.OrganizationID, Status: model.ShareRequestApproved}, nil
		},
	}
	router, token, _, memberID := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{}, handler)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/filesystem/share-requests/"+requestID.String()+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	req.Header.Set("Idempotency-Key", "approve-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Equal(t, orgID, captured.OrganizationID)
	require.Equal(t, memberID.UUID, captured.ActorID)
	require.Equal(t, requestID, captured.RequestID)
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
