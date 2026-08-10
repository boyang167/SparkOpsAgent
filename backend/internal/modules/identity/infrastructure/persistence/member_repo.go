package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MemberRepository struct {
	db *gorm.DB
}

func NewMemberRepository(db *gorm.DB) repository.MemberRepository {
	return &MemberRepository{db: db}
}

func (r *MemberRepository) Save(ctx context.Context, member *model.Member) error {
	row := toMemberModel(member)
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *MemberRepository) FindByEmail(ctx context.Context, email string) (*model.Member, error) {
	var row memberModel
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return toMemberEntity(row), nil
}

func (r *MemberRepository) FindByID(ctx context.Context, id kernel.MemberID) (*model.Member, error) {
	var row memberModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return toMemberEntity(row), nil
}

func (r *MemberRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&memberModel{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (r *MemberRepository) ListAll(ctx context.Context) ([]*model.Member, error) {
	var rows []memberModel
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMemberEntity(row))
	}
	return out, nil
}

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS identity").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&memberModel{})
}

type memberModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	Email        string `gorm:"column:email;uniqueIndex"`
	DisplayName  string `gorm:"column:display_name"`
	PasswordHash string `gorm:"column:password_hash"`
	Status       string `gorm:"column:status"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (memberModel) TableName() string {
	return "identity.members"
}

func toMemberModel(m *model.Member) memberModel {
	return memberModel{
		ID:           m.ID.String(),
		Email:        m.Email,
		DisplayName:  m.DisplayName,
		PasswordHash: m.PasswordHash,
		Status:       string(m.Status),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func toMemberEntity(row memberModel) *model.Member {
	id, _ := kernel.ParseMemberID(row.ID)
	return &model.Member{
		ID:           id,
		Email:        row.Email,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		Status:       model.MemberStatus(row.Status),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
