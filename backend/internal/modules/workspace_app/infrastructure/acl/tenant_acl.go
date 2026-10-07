package acl

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	tenantdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TenantACL struct{ db *gorm.DB }

func NewTenantACL(db *gorm.DB) *TenantACL { return &TenantACL{db: db} }

func (a *TenantACL) MemberInWorkspace(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("tenant.workspace_members").
		Where(postgres.UUIDEq("workspace_id")+" AND "+postgres.UUIDEq("member_id"), wsID.String(), memberID.String()).
		Count(&count).Error
	return count > 0, err
}

func (a *TenantACL) CanCreateProject(ctx context.Context, wsID kernel.WorkspaceID, currentCount int) error {
	var row struct {
		MaxProjects int
		OrgID       string
	}
	err := a.db.WithContext(ctx).Table("tenant.workspaces").
		Select("max_projects, org_id").
		Where(postgres.UUIDEq("id"), wsID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantdomain.ErrWorkspaceNotFound
	}
	if err != nil {
		return err
	}
	quota := model.WorkspaceQuota{MaxProjects: row.MaxProjects}
	ws := &model.Workspace{Quota: quota}
	if currentCount >= ws.Quota.MaxProjects {
		return tenantdomain.ErrQuotaExceeded
	}
	orgID, err := kernel.ParseOrgID(row.OrgID)
	if err != nil {
		return err
	}
	var orgRow struct {
		MaxProjects int
	}
	err = a.db.WithContext(ctx).Table("tenant.organizations").
		Select("max_projects").
		Where(postgres.UUIDEq("id"), orgID.String()).
		First(&orgRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantdomain.ErrOrganizationNotFound
	}
	if err != nil {
		return err
	}
	orgQuota := model.OrgQuota{MaxProjects: orgRow.MaxProjects}
	if orgQuota.MaxProjects <= 0 {
		orgQuota.MaxProjects = model.DefaultOrgQuota().MaxProjects
	}
	var orgProjectCount int64
	err = a.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM workspace_app.projects p
		JOIN tenant.workspaces w ON w.id = p.workspace_id
		WHERE w.org_id::uuid = ?::uuid AND p.status = 'active' AND p.deleted_at IS NULL
	`, orgID.String()).Scan(&orgProjectCount).Error
	if err != nil {
		return err
	}
	return orgQuota.CanCreateProject(int(orgProjectCount))
}

func (a *TenantACL) OrgIDForWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (kernel.OrgID, error) {
	var row struct {
		OrgID string
	}
	err := a.db.WithContext(ctx).Table("tenant.workspaces").
		Select("org_id").
		Where(postgres.UUIDEq("id")+" AND deleted_at IS NULL", wsID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return kernel.OrgID{}, tenantdomain.ErrWorkspaceNotFound
	}
	if err != nil {
		return kernel.OrgID{}, err
	}
	if strings.TrimSpace(row.OrgID) == "" {
		return kernel.OrgID{}, nil
	}
	return kernel.ParseOrgID(row.OrgID)
}

var _ port.WorkspaceChecker = (*TenantACL)(nil)
