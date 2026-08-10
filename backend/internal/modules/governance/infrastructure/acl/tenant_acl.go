package acl

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TenantACL struct{ db *gorm.DB }

func NewTenantACL(db *gorm.DB) *TenantACL { return &TenantACL{db: db} }

func (a *TenantACL) MemberInWorkspace(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("tenant.workspace_members").
		Where("workspace_id = ? AND member_id = ?", wsID.String(), memberID.String()).
		Count(&count).Error
	return count > 0, err
}

func (a *TenantACL) AddWorkspaceMember(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, role string) error {
	now := time.Now().UTC()
	row := struct {
		WorkspaceID string
		MemberID    string
		Role        string
		JoinedAt    time.Time
		postgres.AuditColumns
	}{
		WorkspaceID: wsID.String(),
		MemberID:    memberID.String(),
		Role:        role,
		JoinedAt:    now,
		AuditColumns: postgres.AuditColumns{CreatedAt: now, UpdatedAt: now},
	}
	return a.db.WithContext(ctx).Table("tenant.workspace_members").Save(&row).Error
}
