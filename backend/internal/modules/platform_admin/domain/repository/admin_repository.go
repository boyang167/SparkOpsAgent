package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SysConfig struct {
	ID           string
	Kind         string
	Name         string
	WorkspaceID  *kernel.WorkspaceID
	WhetherShare bool
	Config       map[string]any
}

type ClusterInfo struct {
	ID                       kernel.ClusterInfoID
	ClusterName              string
	Server                   string
	UserName                 string
	CertificateAuthorityData string
	ClientCertificateData    string
	ClientKeyData            string
	BearerToken              string
	IsBuildEnv               bool
	PriorityID               int
	CreatedBy                kernel.MemberID
}

type Proclaim struct {
	ID              kernel.ProclaimID
	Title           string
	Body            string
	ProclaimType    string
	ProclaimReceive string
	Status          string
	CreatedBy       kernel.MemberID
}

type AdminRepository interface {
	GetSysConfig(ctx context.Context) (*SysConfig, error)
	UpsertSysConfig(ctx context.Context, cfg *SysConfig) error
	ListClusters(ctx context.Context) ([]*ClusterInfo, error)
	FindClusterByName(ctx context.Context, clusterName string) (*ClusterInfo, error)
	FindClusterByID(ctx context.Context, clusterID kernel.ClusterInfoID) (*ClusterInfo, error)
	SaveCluster(ctx context.Context, cluster *ClusterInfo) error
	GrantWorkspaceCluster(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID, createdBy kernel.MemberID) error
	RevokeWorkspaceCluster(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) error
	ListWorkspaceClusters(ctx context.Context, wsID kernel.WorkspaceID) ([]*ClusterInfo, error)
	WorkspaceClusterGranted(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (bool, error)
	GetStatisticsCache(ctx context.Context, wsID kernel.WorkspaceID, key string) (map[string]any, error)
	UpsertStatisticsCache(ctx context.Context, wsID kernel.WorkspaceID, key string, payload map[string]any) error
	ListProclaims(ctx context.Context) ([]*Proclaim, error)
	SaveProclaim(ctx context.Context, proclaim *Proclaim) error
	CountProjectsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error)
	CountProductsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error)
	ReorderWorkspaceClusters(ctx context.Context, wsID kernel.WorkspaceID, clusterIDs []kernel.ClusterInfoID) error
}
