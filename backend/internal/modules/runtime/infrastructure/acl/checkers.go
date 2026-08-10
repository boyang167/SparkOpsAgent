package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceAppACL struct{ db *gorm.DB }

func NewWorkspaceAppACL(db *gorm.DB) *WorkspaceAppACL { return &WorkspaceAppACL{db: db} }

func (a *WorkspaceAppACL) ProjectInWorkspace(ctx context.Context, projectID kernel.ProjectID, wsID kernel.WorkspaceID) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("workspace_app.projects").
		Where("id = ? AND workspace_id = ?", projectID.String(), wsID.String()).
		Count(&count).Error
	return count > 0, err
}

func (a *WorkspaceAppACL) ProjectName(ctx context.Context, projectID kernel.ProjectID) (string, error) {
	var row struct {
		Name string
	}
	err := a.db.WithContext(ctx).Table("workspace_app.projects").
		Select("name").
		Where("id = ?", projectID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	return row.Name, err
}

func (a *WorkspaceAppACL) ProjectNamespace(
	ctx context.Context,
	projectID kernel.ProjectID,
	projectName string,
) (string, error) {
	var row struct {
		LegacyNamespace string
	}
	err := a.db.WithContext(ctx).
		Table("workspace_app.projects p").
		Joins("JOIN tenant.workspaces w ON w.id = p.workspace_id").
		Select("w.legacy_namespace").
		Where("p.id = ?", projectID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if err != nil {
		return "", err
	}
	if row.LegacyNamespace != "" {
		return row.LegacyNamespace, nil
	}
	return k8s.DefaultNamespace(projectName), nil
}

var _ port.ProjectChecker = (*WorkspaceAppACL)(nil)
