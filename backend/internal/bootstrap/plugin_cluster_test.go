package bootstrap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain/repository"
	adminacl "github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/infrastructure/acl"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type fakeAdminClusterRepo struct {
	clusters map[kernel.ClusterInfoID]*repository.ClusterInfo
	grants   map[kernel.WorkspaceID]map[kernel.ClusterInfoID]bool
}

func (r fakeAdminClusterRepo) GetSysConfig(context.Context) (*repository.SysConfig, error) {
	return nil, nil
}
func (r fakeAdminClusterRepo) UpsertSysConfig(context.Context, *repository.SysConfig) error {
	return nil
}
func (r fakeAdminClusterRepo) ListClusters(context.Context) ([]*repository.ClusterInfo, error) {
	return nil, nil
}
func (r fakeAdminClusterRepo) FindClusterByName(_ context.Context, clusterName string) (*repository.ClusterInfo, error) {
	for _, cluster := range r.clusters {
		if cluster != nil && cluster.ClusterName == clusterName {
			return cluster, nil
		}
	}
	return nil, nil
}
func (r fakeAdminClusterRepo) FindClusterByID(_ context.Context, clusterID kernel.ClusterInfoID) (*repository.ClusterInfo, error) {
	if cluster, ok := r.clusters[clusterID]; ok {
		return cluster, nil
	}
	return nil, nil
}
func (r fakeAdminClusterRepo) SaveCluster(context.Context, *repository.ClusterInfo) error { return nil }
func (r fakeAdminClusterRepo) GrantWorkspaceCluster(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID, kernel.MemberID) error {
	return nil
}
func (r fakeAdminClusterRepo) RevokeWorkspaceCluster(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) error {
	return nil
}
func (r fakeAdminClusterRepo) ListWorkspaceClusters(_ context.Context, wsID kernel.WorkspaceID) ([]*repository.ClusterInfo, error) {
	var out []*repository.ClusterInfo
	for clusterID := range r.grants[wsID] {
		out = append(out, r.clusters[clusterID])
	}
	return out, nil
}
func (r fakeAdminClusterRepo) WorkspaceClusterGranted(_ context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (bool, error) {
	return r.grants[wsID][clusterID], nil
}
func (r fakeAdminClusterRepo) GetStatisticsCache(context.Context, kernel.WorkspaceID, string) (map[string]any, error) {
	return nil, nil
}
func (r fakeAdminClusterRepo) UpsertStatisticsCache(context.Context, kernel.WorkspaceID, string, map[string]any) error {
	return nil
}
func (r fakeAdminClusterRepo) ListProclaims(context.Context) ([]*repository.Proclaim, error) {
	return nil, nil
}
func (r fakeAdminClusterRepo) SaveProclaim(context.Context, *repository.Proclaim) error { return nil }
func (r fakeAdminClusterRepo) CountProjectsByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r fakeAdminClusterRepo) CountProductsByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r fakeAdminClusterRepo) ReorderWorkspaceClusters(context.Context, kernel.WorkspaceID, []kernel.ClusterInfoID) error {
	return nil
}

type fakePermissionChecker struct {
	allowed bool
	perm    string
}

func (c *fakePermissionChecker) RequirePermission(context.Context, kernel.WorkspaceID, kernel.MemberID, string) error {
	return nil
}

func (c *fakePermissionChecker) HasPermission(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, perm string) (bool, error) {
	c.perm = perm
	return c.allowed, nil
}

func TestClusterCredentialSourceListsOnlyAuthorizedClusters(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	otherClusterID := kernel.NewClusterInfoID()
	src := adminacl.NewClusterCredentialSource(fakeAdminClusterRepo{
		clusters: map[kernel.ClusterInfoID]*repository.ClusterInfo{
			clusterID:      testCluster(clusterID, "prod"),
			otherClusterID: testCluster(otherClusterID, "other"),
		},
		grants: map[kernel.WorkspaceID]map[kernel.ClusterInfoID]bool{
			wsID: {clusterID: true},
		},
	})

	items, err := src.ListAuthorized(context.Background(), wsID)
	require.NoError(t, err)
	require.Equal(t, []sparkplugin.ClusterCredential{{
		ID:                       clusterID,
		Name:                     "prod",
		Server:                   "https://prod.example",
		CertificateAuthorityData: "CA_prod",
		ClientCertificateData:    "CERT_prod",
		ClientKeyData:            "KEY_prod",
	}}, items)
}

func TestClusterCredentialSourceGetAuthorizedLoadsFullCredentials(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	full := testCluster(clusterID, "dev")
	src := adminacl.NewClusterCredentialSource(fakeAdminClusterRepo{
		clusters: map[kernel.ClusterInfoID]*repository.ClusterInfo{clusterID: full},
		grants:   map[kernel.WorkspaceID]map[kernel.ClusterInfoID]bool{wsID: {clusterID: true}},
	})

	cred, err := src.GetAuthorized(context.Background(), wsID, clusterID)
	require.NoError(t, err)
	require.NotNil(t, cred)
	require.Equal(t, "CERT_dev", cred.ClientCertificateData)
	require.Equal(t, "KEY_dev", cred.ClientKeyData)
}

func TestClusterCredentialSourceRejectsUngrantWithoutLeakingCredentials(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	src := adminacl.NewClusterCredentialSource(fakeAdminClusterRepo{
		clusters: map[kernel.ClusterInfoID]*repository.ClusterInfo{clusterID: testCluster(clusterID, "prod")},
		grants:   map[kernel.WorkspaceID]map[kernel.ClusterInfoID]bool{wsID: {}},
	})

	cred, err := src.GetAuthorized(context.Background(), wsID, clusterID)
	require.Nil(t, cred)
	require.Error(t, err)
	domainErr, ok := kernel.IsDomainError(err)
	require.True(t, ok)
	require.Equal(t, "PLUGIN_CLUSTER_FORBIDDEN", domainErr.Code())
	require.NotContains(t, err.Error(), "CA_prod")
	require.NotContains(t, err.Error(), "CERT_prod")
	require.NotContains(t, err.Error(), "KEY_prod")
}

func TestGovernanceWriteAuthorizerChecksGovernanceAdminPermission(t *testing.T) {
	checker := &fakePermissionChecker{allowed: true}
	writer := newGovernanceWriteAuthorizer(checker)

	allowed, err := writer.CanWrite(context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID())

	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, "governance:admin", checker.perm)
}

func testCluster(id kernel.ClusterInfoID, name string) *repository.ClusterInfo {
	return &repository.ClusterInfo{
		ID:                       id,
		ClusterName:              name,
		Server:                   "https://" + name + ".example",
		CertificateAuthorityData: "CA_" + name,
		ClientCertificateData:    "CERT_" + name,
		ClientKeyData:            "KEY_" + name,
	}
}
