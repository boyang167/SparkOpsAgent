package query

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type governanceRepositoryStub struct {
	user      *model.Namespace
	workspace *model.Namespace
	org       *model.Namespace
	shared    []SharedNode
	filter    SharedWithMeFilter
}

func (r *governanceRepositoryStub) GetNamespaceByUser(context.Context, uuid.UUID, uuid.UUID) (*model.Namespace, error) {
	if r.user == nil {
		return nil, domain.ErrNamespaceNotFound
	}
	return r.user, nil
}

func (r *governanceRepositoryStub) GetNamespaceByWorkspace(context.Context, uuid.UUID) (*model.Namespace, error) {
	if r.workspace == nil {
		return nil, domain.ErrNamespaceNotFound
	}
	return r.workspace, nil
}

func (r *governanceRepositoryStub) GetNamespaceByOrganization(context.Context, uuid.UUID) (*model.Namespace, error) {
	if r.org == nil {
		return nil, domain.ErrNamespaceNotFound
	}
	return r.org, nil
}

func (r *governanceRepositoryStub) ListSharedNodes(_ context.Context, _ uuid.UUID, _ []repository.PrincipalRef, filter SharedWithMeFilter, _ time.Time) ([]SharedNode, string, error) {
	r.filter = filter
	return r.shared, "", nil
}

type namespacePrincipalResolver struct {
	principals []repository.PrincipalRef
}

func (r namespacePrincipalResolver) Resolve(context.Context, resourceauth.ResolvePrincipalsInput) ([]repository.PrincipalRef, error) {
	return r.principals, nil
}

type namespaceAuthorizerStub struct {
	allowed map[uuid.UUID]map[model.ResourcePermission]bool
}

func (s namespaceAuthorizerStub) Authorize(_ context.Context, request resourceauth.Request) resourceauth.Decision {
	return resourceauth.Decision{Allowed: s.allowed[request.NodeID][request.Permission], PolicyVersion: 7}
}

func TestListNamespacesReturnsCurrentScopesAndDiscoverableOrganizationSubtree(t *testing.T) {
	orgID, workspaceID, actorID := uuid.New(), uuid.New(), uuid.New()
	userRoot, workspaceRoot, orgEntry := uuid.New(), uuid.New(), uuid.New()
	user := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: orgID, OwnerUserID: actorID, RootNodeID: &userRoot}
	workspace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: orgID, WorkspaceID: workspaceID, RootNodeID: &workspaceRoot}
	organization := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: orgID}
	repo := &governanceRepositoryStub{
		user: user, workspace: workspace, org: organization,
		shared: []SharedNode{{
			Node:             model.Node{ID: orgEntry, NamespaceID: organization.ID},
			EffectiveActions: []model.ResourcePermission{model.ResourcePermissionRead},
		}},
	}
	handler := NewListNamespacesHandler(repo, namespacePrincipalResolver{principals: []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalWorkspace, ID: workspaceID},
	}}, namespaceAuthorizerStub{allowed: map[uuid.UUID]map[model.ResourcePermission]bool{
		userRoot:      {model.ResourcePermissionRead: true, model.ResourcePermissionWrite: true},
		workspaceRoot: {model.ResourcePermissionRead: true},
	}})

	got, err := handler.Handle(context.Background(), ListNamespaces{
		ActorID: actorID, OrganizationID: orgID, WorkspaceID: workspaceID,
	})

	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, model.NamespaceTypeUser, got[0].NamespaceType)
	require.ElementsMatch(t, []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionWrite}, got[0].Capabilities)
	require.Equal(t, model.NamespaceTypeWorkspace, got[1].NamespaceType)
	require.Equal(t, model.NamespaceTypeOrganization, got[2].NamespaceType)
	require.Equal(t, []uuid.UUID{orgEntry}, got[2].EntryNodeIDs)
	require.Equal(t, organization.ID, repo.filter.NamespaceID)
	require.Equal(t, 200, repo.filter.Limit)
}

func TestListNamespacesHidesOrganizationWithoutVisibleNode(t *testing.T) {
	orgID, actorID := uuid.New(), uuid.New()
	userRoot := uuid.New()
	repo := &governanceRepositoryStub{
		user: &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: orgID, OwnerUserID: actorID, RootNodeID: &userRoot},
		org:  &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: orgID},
	}
	handler := NewListNamespacesHandler(repo, namespacePrincipalResolver{}, namespaceAuthorizerStub{})

	got, err := handler.Handle(context.Background(), ListNamespaces{ActorID: actorID, OrganizationID: orgID})

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, model.NamespaceTypeUser, got[0].NamespaceType)
}
