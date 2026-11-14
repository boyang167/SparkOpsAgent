//go:build integration

package security_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	testenv "github.com/sparkops/sparkops-agent/backend/tests/filesystem/integration"
)

func TestWorkspaceCannotReadAnotherOrganizationsBlob(t *testing.T) {
	env := testenv.NewTestEnv(t)
	file := env.UploadAs(t, env.OrganizationA.User, []byte("secret"))

	response := env.DownloadAs(t, env.OrganizationB.User, file.ID)

	require.Equal(t, http.StatusForbidden, response.StatusCode)
	require.NotContains(t, string(response.Body), "secret")
}

type workspaceAccessFixture struct {
	organizationByWorkspace map[uuid.UUID]uuid.UUID
	activeMembers           map[[2]uuid.UUID]bool
}

func (f workspaceAccessFixture) ResolveWorkspaceAccess(_ context.Context, workspaceID, actorID uuid.UUID) (uuid.UUID, bool, error) {
	return f.organizationByWorkspace[workspaceID], f.activeMembers[[2]uuid.UUID{workspaceID, actorID}], nil
}

type roleFixture struct {
	roles map[[2]uuid.UUID][]uuid.UUID
}

func (f roleFixture) ListWorkspaceRoleIDs(_ context.Context, workspaceID, actorID uuid.UUID) ([]uuid.UUID, error) {
	return f.roles[[2]uuid.UUID{workspaceID, actorID}], nil
}

type authorityFixture struct {
	workspaceAdmins    map[[2]uuid.UUID]bool
	organizationAdmins map[[2]uuid.UUID]bool
}

func (f authorityFixture) HasWorkspaceFileAdmin(_ context.Context, workspaceID, actorID uuid.UUID) (bool, error) {
	return f.workspaceAdmins[[2]uuid.UUID{workspaceID, actorID}], nil
}

func (f authorityFixture) HasOrganizationFileAdmin(_ context.Context, organizationID, actorID uuid.UUID) (bool, error) {
	return f.organizationAdmins[[2]uuid.UUID{organizationID, actorID}], nil
}

type authorizationMatrixFixture struct {
	repo        *persistence.Repository
	service     *resourceauth.Service
	orgA        uuid.UUID
	orgB        uuid.UUID
	workspaceA  uuid.UUID
	workspaceB  uuid.UUID
	owner       uuid.UUID
	memberA     uuid.UUID
	memberB     uuid.UUID
	tenantOwner uuid.UUID
	adminRole   uuid.UUID
	memberRole  uuid.UUID
	access      workspaceAccessFixture
	roles       roleFixture
	authority   authorityFixture
}

func newAuthorizationMatrixFixture(t *testing.T) *authorizationMatrixFixture {
	t.Helper()
	testenv.Require(t)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, persistence.AutoMigrate(db))

	fixture := &authorizationMatrixFixture{
		repo:        persistence.NewRepository(db),
		orgA:        uuid.New(),
		orgB:        uuid.New(),
		workspaceA:  uuid.New(),
		workspaceB:  uuid.New(),
		owner:       uuid.New(),
		memberA:     uuid.New(),
		memberB:     uuid.New(),
		tenantOwner: uuid.New(),
		adminRole:   uuid.New(),
		memberRole:  uuid.New(),
		access: workspaceAccessFixture{
			organizationByWorkspace: make(map[uuid.UUID]uuid.UUID),
			activeMembers:           make(map[[2]uuid.UUID]bool),
		},
		roles: roleFixture{roles: make(map[[2]uuid.UUID][]uuid.UUID)},
		authority: authorityFixture{
			workspaceAdmins:    make(map[[2]uuid.UUID]bool),
			organizationAdmins: make(map[[2]uuid.UUID]bool),
		},
	}
	fixture.access.organizationByWorkspace[fixture.workspaceA] = fixture.orgA
	fixture.access.organizationByWorkspace[fixture.workspaceB] = fixture.orgB
	fixture.access.activeMembers[[2]uuid.UUID{fixture.workspaceA, fixture.memberA}] = true
	fixture.access.activeMembers[[2]uuid.UUID{fixture.workspaceB, fixture.memberB}] = true
	fixture.roles.roles[[2]uuid.UUID{fixture.workspaceA, fixture.memberA}] = []uuid.UUID{fixture.memberRole}
	fixture.roles.roles[[2]uuid.UUID{fixture.workspaceB, fixture.memberB}] = []uuid.UUID{fixture.memberRole}
	fixture.authority.organizationAdmins[[2]uuid.UUID{fixture.orgA, fixture.tenantOwner}] = true
	fixture.service = resourceauth.NewService(
		fixture.repo,
		resourceauth.NewPrincipalResolver(fixture.access, fixture.roles),
		fixture.authority,
	)
	return fixture
}

func (f *authorizationMatrixFixture) namespace(t *testing.T, namespaceType model.NamespaceType, organizationID, workspaceID, ownerID uuid.UUID) (*model.Namespace, *model.Node) {
	t.Helper()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: namespaceType, OrganizationID: organizationID,
		WorkspaceID: workspaceID, OwnerUserID: ownerID, Status: "ACTIVE", PolicyVersion: 1,
	}
	require.NoError(t, f.repo.CreateNamespace(context.Background(), namespace))
	root := &model.Node{
		ID: uuid.New(), NamespaceID: namespace.ID, Type: model.NodeTypeDirectory,
		Name: ".root", NormalizedName: ".root", State: model.NodeStateActive,
		OwnerID: f.owner, Classification: model.TransferClassificationInternal,
	}
	require.NoError(t, f.repo.CreateNode(context.Background(), root))
	return namespace, root
}

func (f *authorizationMatrixFixture) child(t *testing.T, namespaceID, parentID uuid.UUID, name string) *model.Node {
	t.Helper()
	child := &model.Node{
		ID: uuid.New(), NamespaceID: namespaceID, ParentID: &parentID,
		Type: model.NodeTypeDirectory, Name: name, NormalizedName: name,
		State: model.NodeStateActive, OwnerID: f.owner,
		Classification: model.TransferClassificationInternal,
	}
	require.NoError(t, f.repo.CreateNode(context.Background(), child))
	return child
}

func (f *authorizationMatrixFixture) grant(t *testing.T, organizationID, nodeID uuid.UUID, principalType model.PrincipalType, principalID uuid.UUID, permission model.ResourcePermission, effect model.GrantEffect, inherit bool) uuid.UUID {
	t.Helper()
	grantID := uuid.New()
	require.NoError(t, f.repo.CreateResourceGrant(context.Background(), model.ResourceGrant{
		ID: grantID, NodeID: nodeID, OrganizationID: organizationID,
		GranteeID: principalID, PrincipalType: principalType, PrincipalID: principalID,
		Permission: permission, Effect: effect, InheritToChildren: inherit, PolicyVersion: 1,
	}))
	return grantID
}

func (f *authorizationMatrixFixture) authorize(actorID, workspaceID, namespaceID, nodeID uuid.UUID, permission model.ResourcePermission) resourceauth.Decision {
	return f.service.Authorize(context.Background(), resourceauth.Request{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespaceID,
		NodeID: nodeID, Permission: permission,
	})
}

func TestEnterpriseAuthorizationMatrixUsesResolverAuthorizerAndRepository(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, *authorizationMatrixFixture)
	}{
		{"tenant owner administers organization namespace", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeOrganization, f.orgA, uuid.Nil, uuid.Nil)
			decision := f.authorize(f.tenantOwner, uuid.Nil, namespace.ID, root.ID, model.ResourcePermissionAdmin)
			require.True(t, decision.Allowed)
			require.Equal(t, "admin_bypass", decision.Reason)
		}},
		{"tenant owner cannot read another users private namespace without grant", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeUser, f.orgA, uuid.Nil, f.owner)
			decision := f.authorize(f.tenantOwner, uuid.Nil, namespace.ID, root.ID, model.ResourcePermissionRead)
			require.False(t, decision.Allowed)
			require.Equal(t, "default_deny", decision.Reason)
		}},
		{"workspace member reads public top level directory through workspace grant", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeWorkspace, f.orgA, f.workspaceA, uuid.Nil)
			f.grant(t, f.orgA, root.ID, model.PrincipalWorkspace, f.workspaceA, model.ResourcePermissionRead, model.GrantEffectAllow, true)
			require.True(t, f.authorize(f.memberA, f.workspaceA, namespace.ID, root.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"workspace member is denied admin only directory without admin role", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeWorkspace, f.orgA, f.workspaceA, uuid.Nil)
			adminOnly := f.child(t, namespace.ID, root.ID, "admin-only")
			f.grant(t, f.orgA, adminOnly.ID, model.PrincipalRole, f.adminRole, model.ResourcePermissionRead, model.GrantEffectAllow, false)
			require.False(t, f.authorize(f.memberA, f.workspaceA, namespace.ID, adminOnly.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"workspace A member reads organization directory shared to A", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeOrganization, f.orgA, uuid.Nil, uuid.Nil)
			f.grant(t, f.orgA, root.ID, model.PrincipalWorkspace, f.workspaceA, model.ResourcePermissionRead, model.GrantEffectAllow, false)
			require.True(t, f.authorize(f.memberA, f.workspaceA, namespace.ID, root.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"workspace B member cannot read directory shared only to A", func(t *testing.T, f *authorizationMatrixFixture) {
			f.access.organizationByWorkspace[f.workspaceB] = f.orgA
			namespace, root := f.namespace(t, model.NamespaceTypeOrganization, f.orgA, uuid.Nil, uuid.Nil)
			f.grant(t, f.orgA, root.ID, model.PrincipalWorkspace, f.workspaceA, model.ResourcePermissionRead, model.GrantEffectAllow, false)
			require.False(t, f.authorize(f.memberB, f.workspaceB, namespace.ID, root.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"personal owner reads own private node", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeUser, f.orgA, uuid.Nil, f.owner)
			require.True(t, f.authorize(f.owner, uuid.Nil, namespace.ID, root.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"inherited allow reaches descendant", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeWorkspace, f.orgA, f.workspaceA, uuid.Nil)
			child := f.child(t, namespace.ID, root.ID, "public-child")
			f.grant(t, f.orgA, root.ID, model.PrincipalWorkspace, f.workspaceA, model.ResourcePermissionRead, model.GrantEffectAllow, true)
			require.True(t, f.authorize(f.memberA, f.workspaceA, namespace.ID, child.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"direct deny overrides inherited allow", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeWorkspace, f.orgA, f.workspaceA, uuid.Nil)
			child := f.child(t, namespace.ID, root.ID, "restricted-child")
			f.grant(t, f.orgA, root.ID, model.PrincipalWorkspace, f.workspaceA, model.ResourcePermissionRead, model.GrantEffectAllow, true)
			f.grant(t, f.orgA, child.ID, model.PrincipalUser, f.memberA, model.ResourcePermissionRead, model.GrantEffectDeny, false)
			decision := f.authorize(f.memberA, f.workspaceA, namespace.ID, child.ID, model.ResourcePermissionRead)
			require.False(t, decision.Allowed)
			require.Equal(t, "explicit_deny", decision.Reason)
		}},
		{"revoked recipient is denied immediately", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeWorkspace, f.orgA, f.workspaceA, uuid.Nil)
			grantID := f.grant(t, f.orgA, root.ID, model.PrincipalUser, f.memberA, model.ResourcePermissionRead, model.GrantEffectAllow, false)
			require.True(t, f.authorize(f.memberA, f.workspaceA, namespace.ID, root.ID, model.ResourcePermissionRead).Allowed)
			require.NoError(t, f.repo.RevokeResourceGrant(context.Background(), root.ID, grantID))
			require.False(t, f.authorize(f.memberA, f.workspaceA, namespace.ID, root.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"cross organization workspace recipient is denied", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeOrganization, f.orgA, uuid.Nil, uuid.Nil)
			f.grant(t, f.orgA, root.ID, model.PrincipalUser, f.memberB, model.ResourcePermissionRead, model.GrantEffectAllow, false)
			decision := f.authorize(f.memberB, f.workspaceB, namespace.ID, root.ID, model.ResourcePermissionRead)
			require.False(t, decision.Allowed)
			require.Equal(t, "organization_boundary", decision.Reason)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.run(t, newAuthorizationMatrixFixture(t))
		})
	}
}

var _ repository.FilesystemRepository = (*persistence.Repository)(nil)
