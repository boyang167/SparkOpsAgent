package acl

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceMember struct {
	MemberID    kernel.MemberID
	Email       string
	DisplayName string
}

type MemberACL struct{ db *gorm.DB }

func NewMemberACL(db *gorm.DB) *MemberACL { return &MemberACL{db: db} }

func (a *MemberACL) ListWorkspaceMembers(ctx context.Context, wsID kernel.WorkspaceID) ([]WorkspaceMember, error) {
	var rows []struct {
		MemberID    string
		Email       string
		DisplayName string
	}
	err := a.db.WithContext(ctx).
		Table("tenant.workspace_members wm").
		Select("wm.member_id, m.email, m.display_name").
		Joins("JOIN identity.members m ON m.id = wm.member_id").
		Where("wm.workspace_id = ?", wsID.String()).
		Order("m.email asc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceMember, 0, len(rows))
	for _, row := range rows {
		memberID, err := kernel.ParseMemberID(row.MemberID)
		if err != nil {
			continue
		}
		out = append(out, WorkspaceMember{
			MemberID: memberID, Email: row.Email, DisplayName: row.DisplayName,
		})
	}
	return out, nil
}
