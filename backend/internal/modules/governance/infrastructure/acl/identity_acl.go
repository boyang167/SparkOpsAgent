package acl

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type IdentityMember struct {
	MemberID    kernel.MemberID
	Email       string
	DisplayName string
}

type IdentityACL struct{ db *gorm.DB }

func NewIdentityACL(db *gorm.DB) *IdentityACL { return &IdentityACL{db: db} }

func (a *IdentityACL) FindMemberByEmail(ctx context.Context, email string) (*IdentityMember, error) {
	var row struct {
		ID          string
		Email       string
		DisplayName string
	}
	err := a.db.WithContext(ctx).Table("identity.members").
		Select("id, email, display_name").
		Where("email = ?", email).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	memberID, err := kernel.ParseMemberID(row.ID)
	if err != nil {
		return nil, err
	}
	return &IdentityMember{
		MemberID: memberID, Email: row.Email, DisplayName: row.DisplayName,
	}, nil
}

func (a *IdentityACL) SearchInviteCandidates(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	query string,
	limit int,
) ([]IdentityMember, error) {
	query = strings.TrimSpace(strings.ToLower(query))
	if len(query) < 2 {
		return []IdentityMember{}, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	prefix := query + "%"
	var rows []struct {
		ID          string
		Email       string
		DisplayName string
	}
	// Prefix match on email/display_name + exclude existing workspace members.
	// Uses partial indexes (migration 000030) for 100k+ scale.
	err := a.db.WithContext(ctx).Raw(`
		SELECT m.id, m.email, m.display_name
		FROM identity.members m
		WHERE m.deleted_at IS NULL
		  AND m.status = 'active'
		  AND (
		    lower(m.email) LIKE ?
		    OR lower(m.display_name) LIKE ?
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM tenant.workspace_members wm
		    WHERE wm.workspace_id = ? AND wm.member_id = m.id
		  )
		ORDER BY
		  CASE WHEN lower(m.email) LIKE ? THEN 0 ELSE 1 END,
		  m.email
		LIMIT ?
	`, prefix, prefix, wsID.String(), prefix, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]IdentityMember, 0, len(rows))
	for _, row := range rows {
		memberID, err := kernel.ParseMemberID(row.ID)
		if err != nil {
			continue
		}
		out = append(out, IdentityMember{
			MemberID: memberID, Email: row.Email, DisplayName: row.DisplayName,
		})
	}
	return out, nil
}
