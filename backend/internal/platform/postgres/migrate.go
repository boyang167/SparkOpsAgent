package postgres

import (
	"fmt"

	"gorm.io/gorm"
)

// UUIDEq compares a column to a bound UUID parameter.
// Both sides are cast to uuid so queries work when SQL migrations use uuid
// but legacy GORM AutoMigrate left the column as text.
func UUIDEq(column string) string {
	return fmt.Sprintf("%s::uuid = ?::uuid", column)
}

// SQLMigrationsApplied reports whether golang-migrate SQL files already built the schema.
func SQLMigrationsApplied(db *gorm.DB) bool {
	if db == nil || db.Dialector.Name() != "postgres" {
		return false
	}
	var count int64
	err := db.Raw(`
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'schema_migrations'
	`).Scan(&count).Error
	if err != nil || count == 0 {
		return false
	}
	var row struct {
		Version int  `gorm:"column:version"`
		Dirty   bool `gorm:"column:dirty"`
	}
	if err := db.Raw(`SELECT version, dirty FROM public.schema_migrations LIMIT 1`).Scan(&row).Error; err != nil {
		return false
	}
	return row.Version > 0 && !row.Dirty
}

// ShouldRunAutoMigrate reports whether GORM AutoMigrate should run DDL.
func ShouldRunAutoMigrate(db *gorm.DB) bool {
	return !SQLMigrationsApplied(db)
}
