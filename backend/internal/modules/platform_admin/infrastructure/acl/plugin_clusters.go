package acl

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type clusterCredentialSource struct {
	repo repository.AdminRepository
}

func NewClusterCredentialSource(repo repository.AdminRepository) plugin.ClusterCredentialSource {
	return &clusterCredentialSource{repo: repo}
}

func (s *clusterCredentialSource) ListAuthorized(ctx context.Context, wsID kernel.WorkspaceID) ([]plugin.ClusterCredential, error) {
	clusters, err := s.repo.ListWorkspaceClusters(ctx, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.ClusterCredential, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster == nil {
			continue
		}
		full, err := s.repo.FindClusterByID(ctx, cluster.ID)
		if err != nil {
			return nil, err
		}
		if full == nil {
			continue
		}
		out = append(out, clusterCredentialFromAdmin(full))
	}
	return out, nil
}

func (s *clusterCredentialSource) GetAuthorized(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (*plugin.ClusterCredential, error) {
	granted, err := s.repo.WorkspaceClusterGranted(ctx, wsID, clusterID)
	if err != nil {
		return nil, err
	}
	if !granted {
		return nil, kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
	}
	cluster, err := s.repo.FindClusterByID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
	}
	cred := clusterCredentialFromAdmin(cluster)
	return &cred, nil
}

func clusterCredentialFromAdmin(cluster *repository.ClusterInfo) plugin.ClusterCredential {
	return plugin.ClusterCredential{
		ID:                       cluster.ID,
		Name:                     cluster.ClusterName,
		Server:                   cluster.Server,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
		ClientCertificateData:    cluster.ClientCertificateData,
		ClientKeyData:            cluster.ClientKeyData,
		BearerToken:              cluster.BearerToken,
	}
}
