package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain"
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
	if err := db.Exec("DROP TABLE IF EXISTS platform_admin.sync_jobs").Error; err != nil {
		return err
	}
	return db.AutoMigrate(&sysCfgModel{}, &clusterInfoModel{}, &workspaceClusterModel{}, &statisticsCacheModel{}, &proclaimModel{})
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
	BearerToken              string `gorm:"type:text"`
	IsBuildEnv               bool
	PriorityID               int
	CreatedBy                string
	postgres.AuditColumns
}

func (clusterInfoModel) TableName() string { return "platform_admin.cluster_info" }

type workspaceClusterModel struct {
	WorkspaceID string `gorm:"column:workspace_id;primaryKey"`
	ClusterID   string `gorm:"column:cluster_id;primaryKey"`
	PriorityID  int    `gorm:"column:priority_id;not null;default:0"`
	CreatedBy   string
	CreatedAt   time.Time
}

func (workspaceClusterModel) TableName() string { return "platform_admin.workspace_clusters" }

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

func (r *AdminRepository) FindClusterByName(ctx context.Context, clusterName string) (*repository.ClusterInfo, error) {
	var row clusterInfoModel
	err := r.db.WithContext(ctx).
		Where("cluster_name = ?", strings.TrimSpace(clusterName)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rowToCluster(row), nil
}

func (r *AdminRepository) FindClusterByID(ctx context.Context, clusterID kernel.ClusterInfoID) (*repository.ClusterInfo, error) {
	var row clusterInfoModel
	err := r.db.WithContext(ctx).
		Where("id = ?", clusterID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rowToCluster(row), nil
}

func (r *AdminRepository) SaveCluster(ctx context.Context, cluster *repository.ClusterInfo) error {
	row := clusterInfoModel{
		ID: cluster.ID.String(), ClusterName: cluster.ClusterName, Server: cluster.Server,
		UserName: cluster.UserName, CertificateAuthorityData: cluster.CertificateAuthorityData,
		ClientCertificateData: cluster.ClientCertificateData, ClientKeyData: cluster.ClientKeyData,
		BearerToken: cluster.BearerToken,
		IsBuildEnv:  cluster.IsBuildEnv, PriorityID: cluster.PriorityID, CreatedBy: cluster.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *AdminRepository) GrantWorkspaceCluster(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID, createdBy kernel.MemberID) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&clusterInfoModel{}).
		Where("id = ?", clusterID.String()).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrClusterNotFound
	}
	row := workspaceClusterModel{
		WorkspaceID: wsID.String(), ClusterID: clusterID.String(),
		CreatedBy: createdBy.String(), CreatedAt: time.Now().UTC(),
	}
	var maxPriority int
	if err := r.db.WithContext(ctx).Model(&workspaceClusterModel{}).
		Where("workspace_id = ?", wsID.String()).
		Select("COALESCE(MAX(priority_id), -1)").
		Scan(&maxPriority).Error; err != nil {
		return err
	}
	row.PriorityID = maxPriority + 1
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (r *AdminRepository) RevokeWorkspaceCluster(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) error {
	return r.db.WithContext(ctx).
		Where("workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).
		Delete(&workspaceClusterModel{}).Error
}

func (r *AdminRepository) ListWorkspaceClusters(ctx context.Context, wsID kernel.WorkspaceID) ([]*repository.ClusterInfo, error) {
	var rows []struct {
		ID          string
		ClusterName string
		Server      string
		PriorityID  int
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT c.id, c.cluster_name, c.server, g.priority_id
		FROM platform_admin.cluster_info AS c
		INNER JOIN platform_admin.workspace_clusters AS g ON g.cluster_id = c.id
		WHERE g.workspace_id = ?
		ORDER BY g.priority_id ASC`, wsID.String()).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*repository.ClusterInfo, 0, len(rows))
	for i, row := range rows {
		id, err := kernel.ParseClusterInfoID(row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, &repository.ClusterInfo{
			ID: id, ClusterName: row.ClusterName, Server: row.Server,
			PriorityID: row.PriorityID, IsBuildEnv: i == 0,
		})
	}
	return out, nil
}

func (r *AdminRepository) ReorderWorkspaceClusters(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	clusterIDs []kernel.ClusterInfoID,
) error {
	if len(clusterIDs) == 0 {
		return nil
	}
	var granted int64
	if err := r.db.WithContext(ctx).Model(&workspaceClusterModel{}).
		Where("workspace_id = ?", wsID.String()).
		Count(&granted).Error; err != nil {
		return err
	}
	if int(granted) != len(clusterIDs) {
		return domain.ErrWorkspaceClusterOrderInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for priority, clusterID := range clusterIDs {
			result := tx.Model(&workspaceClusterModel{}).
				Where("workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).
				Update("priority_id", priority)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return domain.ErrWorkspaceClusterOrderInvalid
			}
		}
		return nil
	})
}

func (r *AdminRepository) WorkspaceClusterGranted(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&workspaceClusterModel{}).
		Where("workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).
		Count(&count).Error
	return count > 0, err
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
		ClientCertificateData:    row.ClientCertificateData, ClientKeyData: row.ClientKeyData,
		BearerToken: row.BearerToken,
		IsBuildEnv:  row.IsBuildEnv, PriorityID: row.PriorityID, CreatedBy: createdBy,
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
