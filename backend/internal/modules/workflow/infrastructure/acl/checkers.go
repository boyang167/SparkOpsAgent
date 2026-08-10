package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/port"
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

func (a *WorkspaceAppACL) ProjectIsActive(ctx context.Context, projectID kernel.ProjectID) (bool, error) {
	var row struct {
		Status string
	}
	err := a.db.WithContext(ctx).Table("workspace_app.projects").
		Select("status").
		Where("id = ?", projectID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.Status == "active", nil
}

var _ port.ProjectChecker = (*WorkspaceAppACL)(nil)
