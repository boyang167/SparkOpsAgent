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
	err := a.db.WithContext(ctx).Raw(`
		SELECT wm.member_id, m.email, m.display_name
		FROM tenant.workspace_members wm
		JOIN identity.members m ON m.id::uuid = wm.member_id::uuid
		WHERE wm.workspace_id::uuid = ?::uuid
		  AND m.deleted_at IS NULL
		ORDER BY m.email ASC
	`, wsID.String()).Scan(&rows).Error
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
