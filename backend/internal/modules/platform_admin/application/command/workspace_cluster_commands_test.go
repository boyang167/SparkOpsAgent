package command_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type workspaceClusterRepo struct {
	clusterExists bool
	granted       bool
	revoked       bool
	clusters      []*repository.ClusterInfo
}

func (r *workspaceClusterRepo) GetSysConfig(context.Context) (*repository.SysConfig, error) {
	return nil, nil
}
func (r *workspaceClusterRepo) UpsertSysConfig(context.Context, *repository.SysConfig) error {
	return nil
}
func (r *workspaceClusterRepo) ListClusters(context.Context) ([]*repository.ClusterInfo, error) {
	return nil, nil
}
func (r *workspaceClusterRepo) SaveCluster(context.Context, *repository.ClusterInfo) error {
	return nil
}
func (r *workspaceClusterRepo) GetStatisticsCache(context.Context, kernel.WorkspaceID, string) (map[string]any, error) {
	return nil, nil
}
func (r *workspaceClusterRepo) UpsertStatisticsCache(context.Context, kernel.WorkspaceID, string, map[string]any) error {
	return nil
}
func (r *workspaceClusterRepo) ListProclaims(context.Context) ([]*repository.Proclaim, error) {
	return nil, nil
}
func (r *workspaceClusterRepo) SaveProclaim(context.Context, *repository.Proclaim) error {
	return nil
}
func (r *workspaceClusterRepo) CountProjectsByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r *workspaceClusterRepo) CountProductsByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r *workspaceClusterRepo) GrantWorkspaceCluster(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID, kernel.MemberID) error {
	if !r.clusterExists {
		return domain.ErrClusterNotFound
	}
	r.granted = true
	return nil
}
func (r *workspaceClusterRepo) RevokeWorkspaceCluster(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) error {
	r.revoked = true
	return nil
}
func (r *workspaceClusterRepo) ListWorkspaceClusters(context.Context, kernel.WorkspaceID) ([]*repository.ClusterInfo, error) {
	return r.clusters, nil
}
func (r *workspaceClusterRepo) WorkspaceClusterGranted(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (bool, error) {
	return r.granted, nil
}

func TestGrantWorkspaceClusterRejectsUnknownCluster(t *testing.T) {
	repo := &workspaceClusterRepo{}
	h := command.NewGrantWorkspaceClusterHandler(repo)

	err := h.Handle(context.Background(), kernel.NewMemberID(), kernel.NewWorkspaceID(), kernel.NewClusterInfoID())

	require.ErrorIs(t, err, domain.ErrClusterNotFound)
	require.False(t, repo.granted)
}

func TestGrantWorkspaceClusterGrantsKnownCluster(t *testing.T) {
	repo := &workspaceClusterRepo{clusterExists: true}
	h := command.NewGrantWorkspaceClusterHandler(repo)

	require.NoError(t, h.Handle(context.Background(), kernel.NewMemberID(), kernel.NewWorkspaceID(), kernel.NewClusterInfoID()))
	require.True(t, repo.granted)
}

func TestRevokeWorkspaceCluster(t *testing.T) {
	repo := &workspaceClusterRepo{}
	h := command.NewRevokeWorkspaceClusterHandler(repo)

	require.NoError(t, h.Handle(context.Background(), kernel.NewWorkspaceID(), kernel.NewClusterInfoID()))
	require.True(t, repo.revoked)
}

func TestListWorkspaceClustersReturnsOnlyRepositoryGrants(t *testing.T) {
	granted := &repository.ClusterInfo{
		ID: kernel.NewClusterInfoID(), ClusterName: "production", Server: "https://k8s.example",
		IsBuildEnv: true, PriorityID: 3,
	}
	repo := &workspaceClusterRepo{clusters: []*repository.ClusterInfo{granted}}

	items, err := query.NewListWorkspaceClustersHandler(repo).Handle(context.Background(), kernel.NewWorkspaceID())

	require.NoError(t, err)
	require.Equal(t, []map[string]any{{
		"id": granted.ID.String(), "cluster_name": "production", "server": "https://k8s.example",
		"is_build_env": true, "priority_id": 3,
	}}, items)
}
