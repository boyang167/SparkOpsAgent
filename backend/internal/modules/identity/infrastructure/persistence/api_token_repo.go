package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type apiTokenModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	MemberID     string
	WorkspaceID  *string
	RoleID       *string
	TokenPrefix  string
	Status       string
	ExpiresAt    *time.Time
	Remark       string
	postgres.AuditColumns
}

func (apiTokenModel) TableName() string { return "identity.api_tokens" }

type tokenBlacklistModel struct {
	TokenHash string `gorm:"column:token_hash;primaryKey"`
	CreatedAt time.Time
}

func (tokenBlacklistModel) TableName() string { return "identity.token_blacklist" }

type APITokenRecord struct {
	ID          string     `json:"id"`
	MemberID    string     `json:"member_id"`
	WorkspaceID *string    `json:"workspace_id,omitempty"`
	RoleID      *string    `json:"role_id,omitempty"`
	TokenPrefix string     `json:"token_prefix"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Remark      string     `json:"remark"`
	CreatedAt   time.Time  `json:"created_at"`
}

type APITokenStore struct {
	db *gorm.DB
}

func NewAPITokenStore(db *gorm.DB) *APITokenStore {
	return &APITokenStore{db: db}
}

func (s *APITokenStore) AutoMigrate() error {
	if !postgres.ShouldRunAutoMigrate(s.db) {
		return nil
	}
	return s.db.AutoMigrate(&apiTokenModel{}, &tokenBlacklistModel{})
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *APITokenStore) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&tokenBlacklistModel{}).
		Where("token_hash = ?", HashToken(token)).Count(&count).Error
	return count > 0, err
}

func (s *APITokenStore) Blacklist(ctx context.Context, token string) error {
	row := tokenBlacklistModel{TokenHash: HashToken(token), CreatedAt: time.Now().UTC()}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *APITokenStore) Create(ctx context.Context, record APITokenRecord) error {
	row := apiTokenModel{
		ID: record.ID, MemberID: record.MemberID, WorkspaceID: record.WorkspaceID,
		RoleID: record.RoleID, TokenPrefix: record.TokenPrefix, Status: record.Status,
		ExpiresAt: record.ExpiresAt, Remark: record.Remark,
		AuditColumns: postgres.AuditColumns{CreatedAt: record.CreatedAt, UpdatedAt: record.CreatedAt},
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *APITokenStore) ListByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) ([]APITokenRecord, error) {
	var rows []apiTokenModel
	err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID.String()).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rowsToTokenRecords(rows), nil
}

func (s *APITokenStore) Revoke(ctx context.Context, id string) (*APITokenRecord, error) {
	var row apiTokenModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	row.Status = "revoked"
	row.UpdatedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	records := rowsToTokenRecords([]apiTokenModel{row})
	return &records[0], nil
}

func NewAPITokenID() string { return uuid.NewString() }

func rowsToTokenRecords(rows []apiTokenModel) []APITokenRecord {
	out := make([]APITokenRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, APITokenRecord{
			ID: row.ID, MemberID: row.MemberID, WorkspaceID: row.WorkspaceID,
			RoleID: row.RoleID, TokenPrefix: row.TokenPrefix, Status: row.Status,
			ExpiresAt: row.ExpiresAt, Remark: row.Remark, CreatedAt: row.CreatedAt,
		})
	}
	return out
}
