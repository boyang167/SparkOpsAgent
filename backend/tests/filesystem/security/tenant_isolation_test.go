//go:build integration

package security_test

import (
	"context"
	"encoding/json"
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
	platformauthz "github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	testenv "github.com/sparkops/sparkops-agent/backend/tests/filesystem/integration"
)

func TestWorkspaceCannotReadAnotherOrganizationsBlob(t *testing.T) {
	env := testenv.NewTestEnv(t)
	file := env.UploadAs(t, env.OrganizationA.User, []byte("secret"))

	response := env.DownloadAs(t, env.OrganizationB.User, file.ID)

	require.Equal(t, http.StatusForbidden, response.StatusCode)
	require.NotContains(t, string(response.Body), "secret")
}

type sqliteAuthorityAdapter struct {
	db *gorm.DB
}

func (a sqliteAuthorityAdapter) ResolveWorkspaceAccess(ctx context.Context, workspaceID, actorID uuid.UUID) (uuid.UUID, bool, error) {
	var row struct {
		OrganizationID uuid.UUID
		Status         string
		Active         bool
	}
	err := a.db.WithContext(ctx).Raw(`
		SELECT workspace.org_id AS organization_id, workspace.status,
		       CASE WHEN member.member_id IS NULL THEN FALSE ELSE TRUE END AS active
		FROM tenant.workspaces workspace
		LEFT JOIN tenant.workspace_members member
		  ON member.workspace_id = workspace.id AND member.member_id = ?
		WHERE workspace.id = ?
	`, actorID, workspaceID).Scan(&row).Error
	return row.OrganizationID, row.Status == "ACTIVE" && row.Active, err
}

func (a sqliteAuthorityAdapter) ListWorkspaceRoleIDs(ctx context.Context, workspaceID, actorID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := a.db.WithContext(ctx).Raw(`
		SELECT role.id
		FROM governance.roles role
		JOIN governance.role_bindings binding ON binding.role_id = role.id
		WHERE role.workspace_id = ? AND binding.member_id = ?
		ORDER BY role.id
	`, workspaceID, actorID).Scan(&ids).Error
	return ids, err
}

func (a sqliteAuthorityAdapter) HasWorkspaceFileAdmin(ctx context.Context, workspaceID, actorID uuid.UUID) (bool, error) {
	_, active, err := a.ResolveWorkspaceAccess(ctx, workspaceID, actorID)
	if err != nil || !active {
		return false, err
	}
	var permissions [][]byte
	err = a.db.WithContext(ctx).Raw(`
		SELECT role.permissions
		FROM governance.roles role
		JOIN governance.role_bindings binding ON binding.role_id = role.id
		WHERE role.workspace_id = ? AND binding.member_id = ?
	`, workspaceID, actorID).Scan(&permissions).Error
	if err != nil {
		return false, err
	}
	for _, raw := range permissions {
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return false, err
		}
		for _, permission := range values {
			if permission == platformauthz.PermFileAdmin {
				return true, nil
			}
		}
	}
	return false, nil
}

func (a sqliteAuthorityAdapter) HasOrganizationFileAdmin(ctx context.Context, organizationID, actorID uuid.UUID) (bool, error) {
	var role string
	err := a.db.WithContext(ctx).Raw(`
		SELECT role FROM tenant.organization_members
		WHERE org_id = ? AND member_id = ?
	`, organizationID, actorID).Scan(&role).Error
	if err != nil {
		return false, err
	}
	return platformauthz.OrgRoleHasPermission(role, platformauthz.PermOrgFileAdmin), nil
}

type authorizationMatrixFixture struct {
	repo             *persistence.Repository
	service          *resourceauth.Service
	orgA             uuid.UUID
	orgB             uuid.UUID
	workspaceA       uuid.UUID
	workspaceB       uuid.UUID
	workspaceForeign uuid.UUID
	owner            uuid.UUID
	memberA          uuid.UUID
	memberB          uuid.UUID
	tenantOwner      uuid.UUID
	adminRole        uuid.UUID
	memberRole       uuid.UUID
	authority        sqliteAuthorityAdapter
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
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS tenant").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS governance").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	require.NoError(t, db.Exec(`
		CREATE TABLE tenant.organizations (id BLOB PRIMARY KEY);
		CREATE TABLE tenant.organization_members (
			org_id BLOB NOT NULL,
			member_id BLOB NOT NULL,
			role TEXT NOT NULL,
			PRIMARY KEY (org_id, member_id)
		);
		CREATE TABLE tenant.workspaces (
			id BLOB PRIMARY KEY,
			org_id BLOB NOT NULL,
			status TEXT NOT NULL
		);
		CREATE TABLE tenant.workspace_members (
			workspace_id BLOB NOT NULL,
			member_id BLOB NOT NULL,
			role TEXT NOT NULL,
			PRIMARY KEY (workspace_id, member_id)
		);
		CREATE TABLE governance.roles (
			id BLOB PRIMARY KEY,
			workspace_id BLOB NOT NULL,
			name TEXT NOT NULL,
			permissions BLOB NOT NULL
		);
		CREATE TABLE governance.role_bindings (
			role_id BLOB NOT NULL,
			member_id BLOB NOT NULL,
			PRIMARY KEY (role_id, member_id)
		);
	`).Error)

	fixture := &authorizationMatrixFixture{
		repo:             persistence.NewRepository(db),
		orgA:             uuid.New(),
		orgB:             uuid.New(),
		workspaceA:       uuid.New(),
		workspaceB:       uuid.New(),
		workspaceForeign: uuid.New(),
		owner:            uuid.New(),
		memberA:          uuid.New(),
		memberB:          uuid.New(),
		tenantOwner:      uuid.New(),
		adminRole:        uuid.New(),
		memberRole:       uuid.New(),
		authority:        sqliteAuthorityAdapter{db: db},
	}
	require.NoError(t, db.Exec(
		`INSERT INTO tenant.organizations (id) VALUES (?), (?)`,
		fixture.orgA, fixture.orgB,
	).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tenant.organization_members (org_id, member_id, role)
		VALUES (?, ?, 'owner'), (?, ?, 'member'), (?, ?, 'member'), (?, ?, 'member')
	`,
		fixture.orgA, fixture.tenantOwner,
		fixture.orgA, fixture.memberA,
		fixture.orgA, fixture.memberB,
		fixture.orgB, fixture.memberB,
	).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tenant.workspaces (id, org_id, status)
		VALUES (?, ?, 'ACTIVE'), (?, ?, 'ACTIVE'), (?, ?, 'ACTIVE')
	`,
		fixture.workspaceA, fixture.orgA,
		fixture.workspaceB, fixture.orgA,
		fixture.workspaceForeign, fixture.orgB,
	).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO tenant.workspace_members (workspace_id, member_id, role)
		VALUES (?, ?, 'member'), (?, ?, 'member'), (?, ?, 'member')
	`,
		fixture.workspaceA, fixture.memberA,
		fixture.workspaceB, fixture.memberB,
		fixture.workspaceForeign, fixture.memberB,
	).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO governance.roles (id, workspace_id, name, permissions)
		VALUES (?, ?, 'member', '[]'), (?, ?, 'admin', ?)
	`,
		fixture.memberRole, fixture.workspaceA,
		fixture.adminRole, fixture.workspaceA, `["`+platformauthz.PermFileAdmin+`"]`,
	).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO governance.role_bindings (role_id, member_id)
		VALUES (?, ?), (?, ?)
	`,
		fixture.memberRole, fixture.memberA,
		fixture.memberRole, fixture.memberB,
	).Error)
	fixture.service = resourceauth.NewService(
		fixture.repo,
		resourceauth.NewPrincipalResolver(fixture.authority, fixture.authority),
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
		{"workspace member reads top level directory through membership audience grant", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeWorkspace, f.orgA, f.workspaceA, uuid.Nil)
			f.grant(t, f.orgA, root.ID, model.PrincipalWorkspace, f.workspaceA, model.ResourcePermissionRead, model.GrantEffectAllow, true)
			require.Equal(t, model.TransferClassificationInternal, root.Classification)
			require.True(t, f.authorize(f.memberA, f.workspaceA, namespace.ID, root.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"workspace member is explicitly denied admin only directory", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeWorkspace, f.orgA, f.workspaceA, uuid.Nil)
			adminOnly := f.child(t, namespace.ID, root.ID, "admin-only")
			f.grant(t, f.orgA, adminOnly.ID, model.PrincipalRole, f.adminRole, model.ResourcePermissionRead, model.GrantEffectAllow, false)
			f.grant(t, f.orgA, adminOnly.ID, model.PrincipalUser, f.memberA, model.ResourcePermissionRead, model.GrantEffectDeny, false)
			decision := f.authorize(f.memberA, f.workspaceA, namespace.ID, adminOnly.ID, model.ResourcePermissionRead)
			require.False(t, decision.Allowed)
			require.Equal(t, "explicit_deny", decision.Reason)
		}},
		{"workspace A member reads organization directory shared to A", func(t *testing.T, f *authorizationMatrixFixture) {
			namespace, root := f.namespace(t, model.NamespaceTypeOrganization, f.orgA, uuid.Nil, uuid.Nil)
			f.grant(t, f.orgA, root.ID, model.PrincipalWorkspace, f.workspaceA, model.ResourcePermissionRead, model.GrantEffectAllow, false)
			require.True(t, f.authorize(f.memberA, f.workspaceA, namespace.ID, root.ID, model.ResourcePermissionRead).Allowed)
		}},
		{"workspace B member cannot read directory shared only to A", func(t *testing.T, f *authorizationMatrixFixture) {
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
			decision := f.authorize(f.memberB, f.workspaceForeign, namespace.ID, root.ID, model.ResourcePermissionRead)
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
