package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS platform_admin").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&sysCfgModel{}, &clusterInfoModel{}, &syncJobModel{}, &statisticsCacheModel{}, &proclaimModel{})
}

type sysCfgModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	Kind         string
	Name         string
	WorkspaceID  *string
	WhetherShare bool
	Config       string `gorm:"type:jsonb"`
	postgres.AuditColumns
}

func (sysCfgModel) TableName() string { return "platform_admin.sys_cfg" }

type clusterInfoModel struct {
	ID                       string `gorm:"column:id;primaryKey"`
	ClusterName              string `gorm:"uniqueIndex"`
	Server                   string
	UserName                 string
	CertificateAuthorityData string `gorm:"type:text"`
	ClientCertificateData    string `gorm:"type:text"`
	ClientKeyData            string `gorm:"type:text"`
	IsBuildEnv               bool
	PriorityID               int
	CreatedBy                string
	postgres.AuditColumns
}

func (clusterInfoModel) TableName() string { return "platform_admin.cluster_info" }

type syncJobModel struct {
	ID             string `gorm:"column:id;primaryKey"`
	Name           string
	WorkspaceID    string
	Version        string
	SourceCluster  string
	TargetCluster  string
	SyncResources  string `gorm:"type:text"`
	Status         string
	Result         string `gorm:"type:text"`
	CreatedBy      string
	postgres.AuditColumns
}

func (syncJobModel) TableName() string { return "platform_admin.sync_jobs" }

type statisticsCacheModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	WorkspaceID string
	CacheKey    string
	Payload     string `gorm:"type:jsonb"`
	ExpiresAt   *time.Time
	postgres.AuditColumns
}

func (statisticsCacheModel) TableName() string { return "platform_admin.statistics_cache" }

type proclaimModel struct {
	ID              string `gorm:"column:id;primaryKey"`
	Title           string
	Body            string `gorm:"type:text"`
	ProclaimType    string
	ProclaimReceive string
	Status          string
	CreatedBy       string
	postgres.AuditColumns
}

func (proclaimModel) TableName() string { return "platform_admin.proclaims" }

type AdminRepository struct{ db *gorm.DB }

func NewAdminRepository(db *gorm.DB) repository.AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) GetSysConfig(ctx context.Context) (*repository.SysConfig, error) {
	var row sysCfgModel
	err := r.db.WithContext(ctx).Order("updated_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &repository.SysConfig{Kind: "default", Name: "default", Config: map[string]any{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return rowToSysConfig(row)
}

func (r *AdminRepository) UpsertSysConfig(ctx context.Context, cfg *repository.SysConfig) error {
	payload, _ := json.Marshal(cfg.Config)
	var wsID *string
	if cfg.WorkspaceID != nil {
		s := cfg.WorkspaceID.String()
		wsID = &s
	}
	id := cfg.ID
	if id == "" {
		id = kernel.NewMemberID().String()
	}
	row := sysCfgModel{
		ID: id, Kind: cfg.Kind, Name: cfg.Name, WorkspaceID: wsID,
		WhetherShare: cfg.WhetherShare, Config: string(payload),
		AuditColumns: postgres.AuditColumns{UpdatedAt: time.Now().UTC()},
	}
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *AdminRepository) ListClusters(ctx context.Context) ([]*repository.ClusterInfo, error) {
	var rows []clusterInfoModel
	if err := r.db.WithContext(ctx).Order("priority_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*repository.ClusterInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToCluster(row))
	}
	return out, nil
}

func (r *AdminRepository) SaveCluster(ctx context.Context, cluster *repository.ClusterInfo) error {
	row := clusterInfoModel{
		ID: cluster.ID.String(), ClusterName: cluster.ClusterName, Server: cluster.Server,
		UserName: cluster.UserName, CertificateAuthorityData: cluster.CertificateAuthorityData,
		ClientCertificateData: cluster.ClientCertificateData, ClientKeyData: cluster.ClientKeyData,
		IsBuildEnv: cluster.IsBuildEnv, PriorityID: cluster.PriorityID, CreatedBy: cluster.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *AdminRepository) ListSyncJobs(ctx context.Context) ([]*repository.SyncJob, error) {
	var rows []syncJobModel
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*repository.SyncJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToSyncJob(row))
	}
	return out, nil
}

func (r *AdminRepository) SaveSyncJob(ctx context.Context, job *repository.SyncJob) error {
	row := syncJobModel{
		ID: job.ID.String(), Name: job.Name, WorkspaceID: job.WorkspaceID.String(),
		Version: job.Version, SourceCluster: job.SourceCluster, TargetCluster: job.TargetCluster,
		SyncResources: job.SyncResources, Status: job.Status, Result: job.Result,
		CreatedBy: job.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *AdminRepository) GetStatisticsCache(ctx context.Context, wsID kernel.WorkspaceID, key string) (map[string]any, error) {
	var row statisticsCacheModel
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND cache_key = ?", wsID.String(), key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if row.Payload != "" {
		_ = json.Unmarshal([]byte(row.Payload), &payload)
	}
	return payload, nil
}

func (r *AdminRepository) UpsertStatisticsCache(ctx context.Context, wsID kernel.WorkspaceID, key string, payload map[string]any) error {
	raw, _ := json.Marshal(payload)
	var existing statisticsCacheModel
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND cache_key = ?", wsID.String(), key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row := statisticsCacheModel{
			ID: kernel.NewMemberID().String(), WorkspaceID: wsID.String(), CacheKey: key, Payload: string(raw),
			AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		}
		return r.db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&statisticsCacheModel{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"payload": string(raw), "updated_at": time.Now().UTC(),
	}).Error
}

func (r *AdminRepository) ListProclaims(ctx context.Context) ([]*repository.Proclaim, error) {
	var rows []proclaimModel
	if err := r.db.WithContext(ctx).Where("status = ?", "active").Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*repository.Proclaim, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToProclaim(row))
	}
	return out, nil
}

func (r *AdminRepository) SaveProclaim(ctx context.Context, proclaim *repository.Proclaim) error {
	row := proclaimModel{
		ID: proclaim.ID.String(), Title: proclaim.Title, Body: proclaim.Body,
		ProclaimType: proclaim.ProclaimType, ProclaimReceive: proclaim.ProclaimReceive,
		Status: proclaim.Status, CreatedBy: proclaim.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *AdminRepository) CountProjectsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("workspace_app.projects").
		Where("workspace_id = ? AND status = ?", wsID.String(), "active").Count(&count).Error
	return int(count), err
}

func (r *AdminRepository) CountProductsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("product.products").
		Where("workspace_id = ?", wsID.String()).Count(&count).Error
	return int(count), err
}

func rowToSysConfig(row sysCfgModel) (*repository.SysConfig, error) {
	cfg := map[string]any{}
	if row.Config != "" {
		_ = json.Unmarshal([]byte(row.Config), &cfg)
	}
	var wsID *kernel.WorkspaceID
	if row.WorkspaceID != nil && *row.WorkspaceID != "" {
		id, _ := kernel.ParseWorkspaceID(*row.WorkspaceID)
		wsID = &id
	}
	return &repository.SysConfig{
		ID: row.ID, Kind: row.Kind, Name: row.Name, WorkspaceID: wsID,
		WhetherShare: row.WhetherShare, Config: cfg,
	}, nil
}

func rowToCluster(row clusterInfoModel) *repository.ClusterInfo {
	id, _ := kernel.ParseClusterInfoID(row.ID)
	createdBy, _ := kernel.ParseMemberID(row.CreatedBy)
	return &repository.ClusterInfo{
		ID: id, ClusterName: row.ClusterName, Server: row.Server, UserName: row.UserName,
		CertificateAuthorityData: row.CertificateAuthorityData,
		ClientCertificateData: row.ClientCertificateData, ClientKeyData: row.ClientKeyData,
		IsBuildEnv: row.IsBuildEnv, PriorityID: row.PriorityID, CreatedBy: createdBy,
	}
}

func rowToSyncJob(row syncJobModel) *repository.SyncJob {
	id, _ := kernel.ParseSyncJobID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	createdBy, _ := kernel.ParseMemberID(row.CreatedBy)
	return &repository.SyncJob{
		ID: id, Name: row.Name, WorkspaceID: wsID, Version: row.Version,
		SourceCluster: row.SourceCluster, TargetCluster: row.TargetCluster,
		SyncResources: row.SyncResources, Status: row.Status, Result: row.Result, CreatedBy: createdBy,
	}
}

func rowToProclaim(row proclaimModel) *repository.Proclaim {
	id, _ := kernel.ParseProclaimID(row.ID)
	createdBy, _ := kernel.ParseMemberID(row.CreatedBy)
	return &repository.Proclaim{
		ID: id, Title: row.Title, Body: row.Body, ProclaimType: row.ProclaimType,
		ProclaimReceive: row.ProclaimReceive, Status: row.Status, CreatedBy: createdBy,
	}
}

var _ repository.AdminRepository = (*AdminRepository)(nil)
