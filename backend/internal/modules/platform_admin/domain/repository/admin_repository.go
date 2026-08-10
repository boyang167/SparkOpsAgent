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
	IsBuildEnv               bool
	PriorityID               int
	CreatedBy                kernel.MemberID
}

type SyncJob struct {
	ID             kernel.SyncJobID
	Name           string
	WorkspaceID    kernel.WorkspaceID
	Version        string
	SourceCluster  string
	TargetCluster  string
	SyncResources  string
	Status         string
	Result         string
	CreatedBy      kernel.MemberID
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
	SaveCluster(ctx context.Context, cluster *ClusterInfo) error
	ListSyncJobs(ctx context.Context) ([]*SyncJob, error)
	SaveSyncJob(ctx context.Context, job *SyncJob) error
	GetStatisticsCache(ctx context.Context, wsID kernel.WorkspaceID, key string) (map[string]any, error)
	UpsertStatisticsCache(ctx context.Context, wsID kernel.WorkspaceID, key string, payload map[string]any) error
	ListProclaims(ctx context.Context) ([]*Proclaim, error)
	SaveProclaim(ctx context.Context, proclaim *Proclaim) error
	CountProjectsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error)
	CountProductsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error)
}
