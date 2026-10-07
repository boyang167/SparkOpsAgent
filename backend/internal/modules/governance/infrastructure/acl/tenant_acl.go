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
		Where(postgres.UUIDEq("workspace_id")+" AND "+postgres.UUIDEq("member_id"), wsID.String(), memberID.String()).
		Count(&count).Error
	return count > 0, err
}

func (a *TenantACL) AddWorkspaceMember(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, role string) error {
	now := time.Now().UTC()
	row := workspaceMemberInsertRow{
		WorkspaceID: wsID.String(),
		MemberID:    memberID.String(),
		Role:        role,
		JoinedAt:    now,
		AuditColumns: postgres.AuditColumns{CreatedAt: now, UpdatedAt: now},
	}
	return a.db.WithContext(ctx).Create(&row).Error
}

type workspaceMemberInsertRow struct {
	WorkspaceID string `gorm:"column:workspace_id;primaryKey;type:uuid"`
	MemberID    string `gorm:"column:member_id;primaryKey;type:uuid"`
	Role        string `gorm:"column:role"`
	JoinedAt    time.Time `gorm:"column:joined_at"`
	postgres.AuditColumns
}

func (workspaceMemberInsertRow) TableName() string { return "tenant.workspace_members" }
