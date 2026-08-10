package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	tenantdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/port"
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

type TenantACL struct{ db *gorm.DB }

func NewTenantACL(db *gorm.DB) *TenantACL { return &TenantACL{db: db} }

func (a *TenantACL) CanCreateAgent(ctx context.Context, wsID kernel.WorkspaceID, currentCount int) error {
	var row struct {
		MaxAgents int
		OrgID     string
	}
	err := a.db.WithContext(ctx).Table("tenant.workspaces").
		Select("max_agents, org_id").
		Where("id = ?", wsID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantdomain.ErrWorkspaceNotFound
	}
	if err != nil {
		return err
	}
	ws := &model.Workspace{Quota: model.WorkspaceQuota{MaxAgents: row.MaxAgents}}
	if currentCount >= ws.Quota.MaxAgents {
		return tenantdomain.ErrQuotaExceeded
	}
	orgID, err := kernel.ParseOrgID(row.OrgID)
	if err != nil {
		return err
	}
	var orgRow struct {
		MaxAgents int
	}
	err = a.db.WithContext(ctx).Table("tenant.organizations").
		Select("max_agents").
		Where("id = ?", orgID.String()).
		First(&orgRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantdomain.ErrOrganizationNotFound
	}
	if err != nil {
		return err
	}
	orgQuota := model.OrgQuota{MaxAgents: orgRow.MaxAgents}
	if orgQuota.MaxAgents <= 0 {
		orgQuota.MaxAgents = model.DefaultOrgQuota().MaxAgents
	}
	var orgAgentCount int64
	err = a.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM agent.agents a
		JOIN workspace_app.projects p ON p.id = a.project_id
		JOIN tenant.workspaces w ON w.id = p.workspace_id
		WHERE w.org_id = ? AND a.status != 'archived' AND a.deleted_at IS NULL
	`, orgID.String()).Scan(&orgAgentCount).Error
	if err != nil {
		return err
	}
	return orgQuota.CanCreateAgent(int(orgAgentCount))
}

var _ port.QuotaChecker = (*TenantACL)(nil)
