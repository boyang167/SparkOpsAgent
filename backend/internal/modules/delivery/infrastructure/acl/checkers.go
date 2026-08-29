package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
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

func (a *WorkspaceAppACL) ProjectNamespace(ctx context.Context, projectID kernel.ProjectID) (string, error) {
	var row struct {
		Namespace string
	}
	err := a.db.WithContext(ctx).Table("workspace_app.projects").
		Select("namespace").
		Where("id = ?", projectID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	return row.Namespace, err
}

func (a *WorkspaceAppACL) ProjectAutomationContext(ctx context.Context, projectID kernel.ProjectID) (port.ProjectAutomationContext, error) {
	var row struct {
		WorkspaceID string
		CreatedBy   string
	}
	err := a.db.WithContext(ctx).Table("workspace_app.projects").
		Select("workspace_id", "created_by").
		Where("id = ?", projectID.String()).
		First(&row).Error
	if err != nil {
		return port.ProjectAutomationContext{}, err
	}
	wsID, err := kernel.ParseWorkspaceID(row.WorkspaceID)
	if err != nil {
		return port.ProjectAutomationContext{}, err
	}
	memberID, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return port.ProjectAutomationContext{}, err
	}
	return port.ProjectAutomationContext{WorkspaceID: wsID, MemberID: memberID}, nil
}

var _ port.ProjectChecker = (*WorkspaceAppACL)(nil)
