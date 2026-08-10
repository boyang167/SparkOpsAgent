package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
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
