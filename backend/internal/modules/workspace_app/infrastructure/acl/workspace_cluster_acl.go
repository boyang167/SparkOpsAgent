package acl

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type WorkspaceClusterACL struct {
	db *gorm.DB
}

func NewWorkspaceClusterACL(db *gorm.DB) *WorkspaceClusterACL {
	return &WorkspaceClusterACL{db: db}
}

func (a *WorkspaceClusterACL) WorkspaceClusterGranted(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("platform_admin.workspace_clusters").
		Where("workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).
		Count(&count).Error
	return count > 0, err
}

var _ port.WorkspaceClusterChecker = (*WorkspaceClusterACL)(nil)
