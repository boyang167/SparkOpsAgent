package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

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
	active    bool
	fileAdmin bool
}

func (s governanceOrgStub) IsOrgMember(context.Context, kernel.OrgID, kernel.MemberID) (bool, error) {
	return s.active, nil
}
func (s governanceOrgStub) HasOrgPermission(context.Context, kernel.OrgID, kernel.MemberID, string) (bool, error) {
	return s.fileAdmin, nil
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
	cursor, nodeID, grantID, principalID := uuid.NewString(), uuid.New(), uuid.New(), uuid.New()
	expiry := time.Date(2026, time.December, 31, 16, 0, 0, 0, time.UTC)
	var captured query.SharedWithMe
	handler := &GovernanceHandler{
		sharedWithMe: func(_ context.Context, input query.SharedWithMe) ([]repository.SharedNode, string, error) {
			captured = input
			return []repository.SharedNode{{
				Node:             model.Node{ID: nodeID, NamespaceID: uuid.New(), Type: model.NodeTypeFile, Name: "报告.md"},
				EffectiveActions: []model.ResourcePermission{model.ResourcePermissionRead},
				SourceGrantIDs:   []uuid.UUID{grantID},
				Sources: []repository.SharedGrantSource{{
					GrantID: grantID, PrincipalType: model.PrincipalAgent,
					PrincipalID: principalID, Permission: model.ResourcePermissionRead,
					Effect: model.GrantEffectAllow, ExpiresAt: &expiry,
				}},
				OriginalPath: "/项目/报告.md",
				SourceNamespace: repository.SharedNamespaceSource{
					NamespaceType:  model.NamespaceTypeWorkspace,
					OrganizationID: orgID,
					WorkspaceID:    &workspaceID,
				},
			}}, "", nil
		},
	}
	router, token, _, memberID := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgID, active: true}, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/shared-with-me?source_type=agent&q=budget&cursor="+cursor+"&limit=23", nil)
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
	require.Equal(t, model.PrincipalAgent, captured.SourceType)
	require.Contains(t, res.Body.String(), `"original_path":"/项目/报告.md"`)
	require.Contains(t, res.Body.String(), `"principal_type":"AGENT"`)
	require.Contains(t, res.Body.String(), `"permission":"READ"`)
	require.Contains(t, res.Body.String(), `"effect":"ALLOW"`)
	require.Contains(t, res.Body.String(), `"expires_at":"2026-12-31T16:00:00Z"`)
	require.Contains(t, res.Body.String(), `"organization_id":"`+orgID.String()+`"`)
	require.Contains(t, res.Body.String(), `"workspace_id":"`+workspaceID.String()+`"`)
}

func TestGovernanceSharedWithMeRejectsUnknownSourceType(t *testing.T) {
	orgID := uuid.New()
	handler := &GovernanceHandler{
		sharedWithMe: func(context.Context, query.SharedWithMe) ([]repository.SharedNode, string, error) {
			t.Fatal("handler must not run for invalid source_type")
			return nil, "", nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{}, handler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/shared-with-me?source_type=unknown", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
}

func TestGovernanceMetadataRequiresAdminAndReturnsRealClassification(t *testing.T) {
	orgID, workspaceID, namespaceID, nodeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	grantID, principalID, requestID := uuid.New(), uuid.New(), uuid.New()
	handler := &GovernanceHandler{
		nodeByID: func(context.Context, uuid.UUID) (*model.Node, error) {
			return &model.Node{
				ID: nodeID, NamespaceID: namespaceID, Type: model.NodeTypeFile,
				Classification: model.TransferClassificationConfidential,
			}, nil
		},
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: orgID, WorkspaceID: workspaceID,
			}, nil
		},
		authorize: func(_ context.Context, request resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Allowed: request.Permission == model.ResourcePermissionAdmin}
		},
		listGovernanceGrants: func(_ context.Context, page repository.GovernanceGrantPageQuery) ([]repository.GovernanceGrantRecord, error) {
			require.Equal(t, 51, page.Limit)
			return []repository.GovernanceGrantRecord{{
				Grant: model.ResourceGrant{
					ID: grantID, NodeID: nodeID, PrincipalType: model.PrincipalWorkspace,
					PrincipalID: principalID, GranteeID: principalID,
					Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
				},
				Source: "DIRECT",
			}}, nil
		},
		listNodeShareRequests: func(_ context.Context, page repository.ShareRequestPageQuery) ([]model.ShareRequest, error) {
			require.Equal(t, 51, page.Limit)
			return []model.ShareRequest{{
				ID: requestID, OrganizationID: orgID, NamespaceID: namespaceID, NodeID: nodeID,
				RequestedBy: principalID, PrincipalType: model.PrincipalWorkspace,
				PrincipalIDs: []uuid.UUID{workspaceID}, Actions: []model.ResourcePermission{model.ResourcePermissionRead},
				Classification: model.TransferClassificationConfidential,
				Status:         model.ShareRequestPending, CreatedAt: time.Now().UTC(),
			}}, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(
		t, orgID, governanceOrgStub{active: true},
		governanceWorkspaceStub{orgID: orgID, active: true}, handler,
	)
	for _, path := range []string{
		"/api/v1/filesystem/nodes/" + nodeID.String() + "/grants",
		"/api/v1/filesystem/nodes/" + nodeID.String() + "/share-requests",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", workspaceID.String())
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		require.Contains(t, res.Body.String(), `"classification":"CONFIDENTIAL"`)
	}
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

func TestSharePreviewAllowsSameOrganizationSourceWorkspaceAndConcealsCrossOrganization(t *testing.T) {
	orgID, otherOrgID := uuid.New(), uuid.New()
	currentWorkspaceID, sourceWorkspaceID := uuid.New(), uuid.New()
	namespaceID, nodeID, principalID, sourceGrantID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	namespaceOrgID := orgID
	previewCalls, submitCalls := 0, 0
	handler := &GovernanceHandler{
		nodeByID: func(context.Context, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: nodeID, NamespaceID: namespaceID}, nil
		},
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: namespaceOrgID, WorkspaceID: sourceWorkspaceID,
			}, nil
		},
		previewShare: func(_ context.Context, cmd command.SubmitShare) (command.SharePreview, error) {
			previewCalls++
			require.Equal(t, currentWorkspaceID, cmd.WorkspaceID)
			require.True(t, cmd.Reshare)
			require.Equal(t, []uuid.UUID{sourceGrantID}, cmd.SourceGrantIDs)
			return command.SharePreview{Outcome: command.SharePreviewActive, Reason: "POLICY_ACTIVE"}, nil
		},
		submitShare: func(_ context.Context, cmd command.SubmitShare) (command.ShareResult, error) {
			submitCalls++
			require.Equal(t, currentWorkspaceID, cmd.WorkspaceID)
			require.True(t, cmd.Reshare)
			require.Equal(t, []uuid.UUID{sourceGrantID}, cmd.SourceGrantIDs)
			return command.ShareResult{
				Request: model.ShareRequest{
					ID: uuid.New(), Status: model.ShareRequestApproved,
					PrincipalType: cmd.PrincipalType, PrincipalIDs: cmd.PrincipalIDs, Actions: cmd.Actions,
				},
				Grants: []model.ResourceGrant{{ID: uuid.New()}},
			}, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(
		t, orgID, governanceOrgStub{active: true},
		governanceWorkspaceStub{orgID: orgID, active: true}, handler,
	)
	send := func() *httptest.ResponseRecorder {
		body := `{"principal_type":"USER","principal_ids":["` + principalID.String() + `"],"actions":["READ"],"reshare":true,"source_grant_ids":["` + sourceGrantID.String() + `"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/filesystem/nodes/"+nodeID.String()+"/share-preview", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", currentWorkspaceID.String())
		req.Header.Set("Idempotency-Key", "preview-1")
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}

	res := send()
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), `"outcome":"ACTIVE"`)
	require.Equal(t, 1, previewCalls)

	body := `{"principal_type":"USER","principal_ids":["` + principalID.String() + `"],"actions":["READ"],"reshare":true,"source_grant_ids":["` + sourceGrantID.String() + `"]}`
	submitReq := httptest.NewRequest(http.MethodPost, "/api/v1/filesystem/nodes/"+nodeID.String()+"/grants", strings.NewReader(body))
	submitReq.Header.Set("Authorization", "Bearer "+token)
	submitReq.Header.Set("X-Organization-ID", orgID.String())
	submitReq.Header.Set("X-Workspace-ID", currentWorkspaceID.String())
	submitReq.Header.Set("Idempotency-Key", "submit-1")
	submitReq.Header.Set("Content-Type", "application/json")
	submitRes := httptest.NewRecorder()
	router.ServeHTTP(submitRes, submitReq)
	require.Equal(t, http.StatusCreated, submitRes.Code, submitRes.Body.String())
	require.Equal(t, 1, submitCalls)

	namespaceOrgID = otherOrgID
	res = send()
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
	require.Equal(t, 1, previewCalls)
	submitReq = httptest.NewRequest(http.MethodPost, "/api/v1/filesystem/nodes/"+nodeID.String()+"/grants", strings.NewReader(body))
	submitReq.Header.Set("Authorization", "Bearer "+token)
	submitReq.Header.Set("X-Organization-ID", orgID.String())
	submitReq.Header.Set("X-Workspace-ID", currentWorkspaceID.String())
	submitReq.Header.Set("Idempotency-Key", "submit-2")
	submitReq.Header.Set("Content-Type", "application/json")
	submitRes = httptest.NewRecorder()
	router.ServeHTTP(submitRes, submitReq)
	require.Equal(t, http.StatusNotFound, submitRes.Code, submitRes.Body.String())
	require.Equal(t, 1, submitCalls)
}

func TestGovernanceMetadataListLimitsAreBounded(t *testing.T) {
	orgID, workspaceID, namespaceID, nodeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	codec, err := query.NewSharedCursorCodec("governance-limit-test", nil)
	require.NoError(t, err)
	expectedRepositoryLimit := 2
	handler := &GovernanceHandler{
		cursor: codec,
		nodeByID: func(context.Context, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: nodeID, NamespaceID: namespaceID}, nil
		},
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: orgID, WorkspaceID: workspaceID,
			}, nil
		},
		authorize: func(context.Context, resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Allowed: true}
		},
		listGovernanceGrants: func(_ context.Context, page repository.GovernanceGrantPageQuery) ([]repository.GovernanceGrantRecord, error) {
			require.Equal(t, expectedRepositoryLimit, page.Limit)
			if expectedRepositoryLimit != 2 {
				return nil, nil
			}
			return []repository.GovernanceGrantRecord{
				{Grant: model.ResourceGrant{ID: uuid.New()}},
				{Grant: model.ResourceGrant{ID: uuid.New()}},
			}, nil
		},
		listNodeShareRequests: func(_ context.Context, page repository.ShareRequestPageQuery) ([]model.ShareRequest, error) {
			require.Equal(t, expectedRepositoryLimit, page.Limit)
			if expectedRepositoryLimit != 2 {
				return nil, nil
			}
			now := time.Now().UTC()
			return []model.ShareRequest{{ID: uuid.New(), CreatedAt: now}, {ID: uuid.New(), CreatedAt: now.Add(-time.Second)}}, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(
		t, orgID, governanceOrgStub{active: true},
		governanceWorkspaceStub{orgID: orgID, active: true}, handler,
	)
	for _, resource := range []string{"grants", "share-requests"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/nodes/"+nodeID.String()+"/"+resource+"?limit=0", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", workspaceID.String())
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())

		req = httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/nodes/"+nodeID.String()+"/"+resource+"?limit=1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", workspaceID.String())
		res = httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		require.Contains(t, res.Body.String(), `"truncated":true`)

		expectedRepositoryLimit = 201
		req = httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/nodes/"+nodeID.String()+"/"+resource+"?limit=999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", workspaceID.String())
		res = httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		expectedRepositoryLimit = 2
	}
}

func TestOrganizationShareRequestPageReturnsAuthoritativeFilteredTotal(t *testing.T) {
	orgID := uuid.New()
	codec, err := query.NewSharedCursorCodec("organization-share-total", nil)
	require.NoError(t, err)
	handler := &GovernanceHandler{
		cursor: codec,
		listNodeShareRequests: func(_ context.Context, page repository.ShareRequestPageQuery) ([]model.ShareRequest, error) {
			require.Equal(t, orgID, page.OrganizationID)
			require.Equal(t, model.ShareRequestPending, page.Status)
			require.Equal(t, 2, page.Limit)
			now := time.Now().UTC()
			return []model.ShareRequest{
				{ID: uuid.New(), OrganizationID: orgID, Status: model.ShareRequestPending, CreatedAt: now},
				{ID: uuid.New(), OrganizationID: orgID, Status: model.ShareRequestPending, CreatedAt: now.Add(-time.Second)},
			}, nil
		},
		countShareRequests: func(_ context.Context, page repository.ShareRequestPageQuery) (int64, error) {
			require.Equal(t, orgID, page.OrganizationID)
			require.Equal(t, model.ShareRequestPending, page.Status)
			require.Zero(t, page.AfterID)
			require.True(t, page.AfterCreatedAt.IsZero())
			return 51, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(
		t, orgID, governanceOrgStub{active: true, fileAdmin: true},
		governanceWorkspaceStub{orgID: orgID, active: true}, handler,
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/share-requests?status=PENDING&limit=1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), `"total_count":51`)
	require.Contains(t, res.Body.String(), `"truncated":true`)
}

func TestOrganizationAuditRequiresFileAdminAndDoesNotApplyWorkspaceFilter(t *testing.T) {
	orgID := uuid.New()
	listed := 0
	handler := &GovernanceHandler{
		listFileAudits: func(_ context.Context, request query.ListFileAuditEvents) (query.ListFileAuditEventsResult, error) {
			listed++
			require.Equal(t, orgID, request.OrganizationID)
			require.Zero(t, request.WorkspaceID)
			return query.ListFileAuditEventsResult{}, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(
		t, orgID, governanceOrgStub{active: true},
		governanceWorkspaceStub{orgID: orgID, active: true}, handler,
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/audits", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusForbidden, res.Code)
	require.Zero(t, listed)

	router, token, _, _ = governanceRouterWithIDs(
		t, orgID, governanceOrgStub{active: true, fileAdmin: true},
		governanceWorkspaceStub{orgID: orgID, active: true}, handler,
	)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/audits", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	require.Equal(t, 1, listed)
}

func TestGovernanceGrantCursorPagesWithoutDuplicatesAndRejectsCrossNodeReplay(t *testing.T) {
	orgID, workspaceID, namespaceID := uuid.New(), uuid.New(), uuid.New()
	nodeID, otherNodeID, firstID, secondID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	codec, err := query.NewSharedCursorCodec("governance-http-cursor", nil)
	require.NoError(t, err)
	handler := &GovernanceHandler{
		cursor: codec,
		nodeByID: func(_ context.Context, requested uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: requested, NamespaceID: namespaceID}, nil
		},
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: orgID, WorkspaceID: workspaceID,
			}, nil
		},
		authorize: func(context.Context, resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Allowed: true}
		},
		listGovernanceGrants: func(_ context.Context, page repository.GovernanceGrantPageQuery) ([]repository.GovernanceGrantRecord, error) {
			require.Equal(t, 2, page.Limit)
			if page.AfterID == uuid.Nil {
				return []repository.GovernanceGrantRecord{
					{Grant: model.ResourceGrant{ID: firstID, NodeID: nodeID}, Source: "DIRECT"},
					{Grant: model.ResourceGrant{ID: secondID, NodeID: nodeID}, Source: "DIRECT"},
				}, nil
			}
			require.Equal(t, firstID, page.AfterID)
			return []repository.GovernanceGrantRecord{
				{Grant: model.ResourceGrant{ID: secondID, NodeID: nodeID}, Source: "DIRECT"},
			}, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(
		t, orgID, governanceOrgStub{active: true},
		governanceWorkspaceStub{orgID: orgID, active: true}, handler,
	)
	get := func(target uuid.UUID, cursor string) *httptest.ResponseRecorder {
		path := "/api/v1/filesystem/nodes/" + target.String() + "/grants?limit=1"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", workspaceID.String())
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}

	first := get(nodeID, "")
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	var firstEnvelope struct {
		Data struct {
			Items      []map[string]any `json:"items"`
			NextCursor string           `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstEnvelope))
	require.Equal(t, firstID.String(), firstEnvelope.Data.Items[0]["id"])
	require.NotEmpty(t, firstEnvelope.Data.NextCursor)

	second := get(nodeID, firstEnvelope.Data.NextCursor)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.NotContains(t, second.Body.String(), firstID.String())
	require.Contains(t, second.Body.String(), secondID.String())

	replayed := get(otherNodeID, firstEnvelope.Data.NextCursor)
	require.Equal(t, http.StatusBadRequest, replayed.Code, replayed.Body.String())
}

func TestGovernanceMetadataRejectsOrdinaryReader(t *testing.T) {
	orgID, workspaceID, namespaceID, nodeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	handler := &GovernanceHandler{
		nodeByID: func(context.Context, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: nodeID, NamespaceID: namespaceID}, nil
		},
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: orgID, WorkspaceID: workspaceID}, nil
		},
		authorize: func(context.Context, resourceauth.Request) resourceauth.Decision {
			return resourceauth.Decision{Allowed: false}
		},
		listGovernanceGrants: func(context.Context, repository.GovernanceGrantPageQuery) ([]repository.GovernanceGrantRecord, error) {
			t.Fatal("ordinary reader must not enumerate grants")
			return nil, nil
		},
		listNodeShareRequests: func(context.Context, repository.ShareRequestPageQuery) ([]model.ShareRequest, error) {
			t.Fatal("ordinary reader must not enumerate share requests")
			return nil, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{orgID: orgID, active: true}, handler)
	for _, resource := range []string{"grants", "share-requests"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/nodes/"+nodeID.String()+"/"+resource, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("X-Workspace-ID", workspaceID.String())
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	}
}

func TestGovernanceMetadataConcealsCrossOrganizationNode(t *testing.T) {
	requestOrgID, sourceOrgID, workspaceID := uuid.New(), uuid.New(), uuid.New()
	namespaceID, nodeID := uuid.New(), uuid.New()
	handler := &GovernanceHandler{
		nodeByID: func(context.Context, uuid.UUID) (*model.Node, error) {
			return &model.Node{ID: nodeID, NamespaceID: namespaceID}, nil
		},
		namespaceByID: func(context.Context, uuid.UUID) (*model.Namespace, error) {
			return &model.Namespace{
				ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
				OrganizationID: sourceOrgID, WorkspaceID: workspaceID,
			}, nil
		},
		authorize: func(context.Context, resourceauth.Request) resourceauth.Decision {
			t.Fatal("cross-organization metadata must be concealed before authorization")
			return resourceauth.Decision{}
		},
	}
	router, token, _, _ := governanceRouterWithIDs(
		t, requestOrgID, governanceOrgStub{active: true},
		governanceWorkspaceStub{orgID: requestOrgID, active: true}, handler,
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/nodes/"+nodeID.String()+"/grants", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", requestOrgID.String())
	req.Header.Set("X-Workspace-ID", workspaceID.String())
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
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

func TestRejectShareRequestHTTPTrimsAndBoundsReason(t *testing.T) {
	orgID, requestID := uuid.New(), uuid.New()
	calls := 0
	var captured command.ReviewShareRequest
	handler := &GovernanceHandler{
		rejectShare: func(_ context.Context, cmd command.ReviewShareRequest) (model.ShareRequest, error) {
			calls++
			captured = cmd
			return model.ShareRequest{ID: cmd.RequestID, Status: model.ShareRequestRejected}, nil
		},
	}
	router, token, _, _ := governanceRouterWithIDs(t, orgID, governanceOrgStub{active: true}, governanceWorkspaceStub{}, handler)
	send := func(reason string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/filesystem/share-requests/"+requestID.String()+"/reject", strings.NewReader(`{"reason":`+strconv.Quote(reason)+`}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Organization-ID", orgID.String())
		req.Header.Set("Idempotency-Key", uuid.NewString())
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	require.Equal(t, http.StatusBadRequest, send("   ").Code)
	require.Equal(t, http.StatusBadRequest, send(strings.Repeat("x", 1001)).Code)
	require.Equal(t, http.StatusBadRequest, send(strings.Repeat("界", 1001)).Code)
	require.Zero(t, calls)
	require.Equal(t, http.StatusOK, send(strings.Repeat("界", 1000)).Code)
	require.Equal(t, strings.Repeat("界", 1000), captured.RejectionReason)
	require.Equal(t, http.StatusOK, send("  policy violation  ").Code)
	require.Equal(t, "policy violation", captured.RejectionReason)
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
