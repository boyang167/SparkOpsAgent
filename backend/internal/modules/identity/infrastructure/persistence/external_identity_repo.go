package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type externalIdentityModel struct {
	Provider string `gorm:"primaryKey"`
	Subject  string `gorm:"primaryKey"`
	MemberID string
	Email    string
	LinkedAt time.Time
}

func (externalIdentityModel) TableName() string { return "identity.external_identities" }

type ExternalIdentityRepository struct{ db *gorm.DB }

func NewExternalIdentityRepository(db *gorm.DB) *ExternalIdentityRepository {
	return &ExternalIdentityRepository{db: db}
}

func (r *ExternalIdentityRepository) AutoMigrate() error {
	if r.db.Dialector.Name() == "postgres" {
		if err := r.db.Exec("CREATE SCHEMA IF NOT EXISTS identity").Error; err != nil {
			return err
		}
	}
	return r.db.AutoMigrate(&externalIdentityModel{})
}

func (r *ExternalIdentityRepository) FindByProviderSubject(ctx context.Context, provider, subject string) (*repository.ExternalIdentity, error) {
	var row externalIdentityModel
	err := r.db.WithContext(ctx).Where("provider = ? AND subject = ?", provider, subject).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	memberID, _ := kernel.ParseMemberID(row.MemberID)
	return &repository.ExternalIdentity{
		Provider: row.Provider, Subject: row.Subject, MemberID: memberID, Email: row.Email,
	}, nil
}

func (r *ExternalIdentityRepository) Save(ctx context.Context, identity *repository.ExternalIdentity) error {
	row := externalIdentityModel{
		Provider: identity.Provider,
		Subject:  identity.Subject,
		MemberID: identity.MemberID.String(),
		Email:    identity.Email,
		LinkedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Save(&row).Error
}

var _ repository.ExternalIdentityRepository = (*ExternalIdentityRepository)(nil)
