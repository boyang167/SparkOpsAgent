package postgres

import (
	"time"

	"gorm.io/gorm"
)

// AuditColumns provides created_at, updated_at, and soft-delete deleted_at for GORM models.
type AuditColumns struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
