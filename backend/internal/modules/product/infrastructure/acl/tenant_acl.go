package acl

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TenantACL struct{ db *gorm.DB }

func NewTenantACL(db *gorm.DB) *TenantACL { return &TenantACL{db: db} }

func (a *TenantACL) MemberInWorkspace(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("tenant.workspace_members").
		Where("workspace_id = ? AND member_id = ?", wsID.String(), memberID.String()).
		Count(&count).Error
	return count > 0, err
}

var _ port.WorkspaceChecker = (*TenantACL)(nil)
