package bootstrap

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type clusterCredentialSource struct {
	repo repository.AdminRepository
}

func newClusterCredentialSource(repo repository.AdminRepository) sparkplugin.ClusterCredentialSource {
	return &clusterCredentialSource{repo: repo}
}

type governanceWriteAuthorizer struct {
	checker authz.PermissionChecker
}

func newGovernanceWriteAuthorizer(checker authz.PermissionChecker) sparkplugin.WriteAuthorizer {
	return governanceWriteAuthorizer{checker: checker}
}

func (a governanceWriteAuthorizer) CanWrite(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error) {
	if a.checker == nil {
		return false, nil
	}
	return a.checker.HasPermission(ctx, wsID, memberID, authz.PermGovernanceAdmin)
}

func (s *clusterCredentialSource) ListAuthorized(ctx context.Context, wsID kernel.WorkspaceID) ([]sparkplugin.ClusterCredential, error) {
	clusters, err := s.repo.ListWorkspaceClusters(ctx, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]sparkplugin.ClusterCredential, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster == nil {
			continue
		}
		out = append(out, clusterCredentialFromAdmin(cluster))
	}
	return out, nil
}

func (s *clusterCredentialSource) GetAuthorized(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (*sparkplugin.ClusterCredential, error) {
	granted, err := s.repo.WorkspaceClusterGranted(ctx, wsID, clusterID)
	if err != nil {
		return nil, err
	}
	if !granted {
		return nil, kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
	}
	clusters, err := s.repo.ListWorkspaceClusters(ctx, wsID)
	if err != nil {
		return nil, err
	}
	for _, cluster := range clusters {
		if cluster == nil || cluster.ID != clusterID {
			continue
		}
		cred := clusterCredentialFromAdmin(cluster)
		return &cred, nil
	}
	return nil, kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
}

func clusterCredentialFromAdmin(cluster *repository.ClusterInfo) sparkplugin.ClusterCredential {
	return sparkplugin.ClusterCredential{
		ID:                       cluster.ID,
		Name:                     cluster.ClusterName,
		Server:                   cluster.Server,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
		ClientCertificateData:    cluster.ClientCertificateData,
		ClientKeyData:            cluster.ClientKeyData,
		BearerToken:              cluster.BearerToken,
	}
}
