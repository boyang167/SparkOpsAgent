package persistence

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type documentFileMigrationModel struct {
	DocumentID   string `gorm:"column:document_id;primaryKey"`
	FileID       *string
	SourceSHA256 string `gorm:"column:source_sha256;not null;size:64"`
	TargetSHA256 string `gorm:"column:target_sha256;size:64"`
	Status       string `gorm:"not null;size:32"`
	ErrorCode    *string
	VerifiedAt   *time.Time
	postgres.AuditColumns
}

func (documentFileMigrationModel) TableName() string { return "knowledge.document_file_migrations" }
