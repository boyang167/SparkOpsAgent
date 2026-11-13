package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	DocumentMigrationStatusLegacy   = "LEGACY"
	DocumentMigrationStatusPrecopy  = "PRECOPY"
	DocumentMigrationStatusSwitched = "SWITCHED"
	DocumentMigrationStatusRollback = "ROLLBACK"
)

const (
	FileMigrationStatusPending  = "PENDING"
	FileMigrationStatusVerified = "VERIFIED"
	FileMigrationStatusFailed   = "FAILED"
)

type DocumentFileMigration struct {
	DocumentID   uuid.UUID
	FileID       *uuid.UUID
	SourceSHA256 string
	TargetSHA256 string
	Status       string
	ErrorCode    *string
	VerifiedAt   *time.Time
}
