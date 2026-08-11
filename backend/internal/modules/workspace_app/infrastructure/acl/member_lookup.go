package acl

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MemberLookup struct{ db *gorm.DB }

func NewMemberLookup(db *gorm.DB) *MemberLookup { return &MemberLookup{db: db} }

func (l *MemberLookup) FindByIDs(ctx context.Context, ids []kernel.MemberID) (map[kernel.MemberID]port.MemberSummary, error) {
	out := make(map[kernel.MemberID]port.MemberSummary)
	if len(ids) == 0 {
		return out, nil
	}
	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		s := id.String()
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		unique = append(unique, s)
	}
	var rows []struct {
		ID          string
		Email       string
		DisplayName string
	}
	if err := l.db.WithContext(ctx).
		Table("identity.members").
		Select("id, email, display_name").
		Where("id IN ?", unique).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		memberID, err := kernel.ParseMemberID(row.ID)
		if err != nil {
			continue
		}
		out[memberID] = port.MemberSummary{
			ID: memberID, Email: row.Email, DisplayName: row.DisplayName,
		}
	}
	return out, nil
}

var _ port.MemberLookup = (*MemberLookup)(nil)
