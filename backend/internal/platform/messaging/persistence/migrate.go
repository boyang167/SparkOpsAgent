package persistence

import (
	"fmt"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		return nil
	}
	if err := db.AutoMigrate(&EventOutbox{}, &EventInbox{}, &OperationOutbox{}); err != nil {
		return err
	}
	return createIndexes(db)
}

const transactionalEventMigrationVersion = 70

// ValidateTransactionalEventSchema requires the official SQL migration on PostgreSQL.
// Non-PostgreSQL databases are used only by unit tests and retain GORM AutoMigrate.
func ValidateTransactionalEventSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is required for transactional event messaging")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}

	var migrations []struct {
		Version int
		Dirty   bool
	}
	result := db.Raw(
		"SELECT version, dirty FROM public.schema_migrations",
	).Scan(&migrations)
	if result.Error != nil {
		return migrationRequiredError("query public.schema_migrations", result.Error)
	}
	if len(migrations) != 1 {
		return migrationRequiredError(
			fmt.Sprintf(
				"public.schema_migrations must contain exactly one row, found %d",
				len(migrations),
			),
			nil,
		)
	}
	migration := migrations[0]
	if migration.Dirty {
		return migrationRequiredError(
			fmt.Sprintf("schema migration version %d is dirty", migration.Version),
			nil,
		)
	}
	if migration.Version < transactionalEventMigrationVersion {
		return migrationRequiredError(
			fmt.Sprintf(
				"schema migration version %d is older than required version %d",
				migration.Version,
				transactionalEventMigrationVersion,
			),
			nil,
		)
	}

	var tableCount int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = 'platform'
		  AND table_name IN ('event_outbox', 'event_inbox', 'operation_outbox')
	`).Scan(&tableCount).Error; err != nil {
		return migrationRequiredError("verify platform transactional event tables", err)
	}
	if tableCount != 3 {
		return migrationRequiredError(
			fmt.Sprintf("expected 3 platform tables, found %d", tableCount),
			nil,
		)
	}
	return nil
}

func migrationRequiredError(reason string, cause error) error {
	message := fmt.Sprintf(
		"transactional event schema is not ready: %s; run `make migrate-up` with DATABASE_URL to apply migration 000070",
		reason,
	)
	if cause == nil {
		return fmt.Errorf("%s", message)
	}
	return fmt.Errorf("%s: %w", message, cause)
}

func createIndexes(db *gorm.DB) error {
	statements := []string{
		"CREATE INDEX IF NOT EXISTS ix_event_outbox_claim ON platform.event_outbox(status, next_attempt_at, created_at)",
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_outbox_idempotency_key ON platform.operation_outbox(idempotency_key)",
		"CREATE INDEX IF NOT EXISTS ix_operation_outbox_claim ON platform.operation_outbox(status, next_attempt_at, created_at)",
	}
	if db.Dialector.Name() == "sqlite" {
		statements = []string{
			"CREATE INDEX IF NOT EXISTS platform.ix_event_outbox_claim ON event_outbox(status, next_attempt_at, created_at)",
			"CREATE UNIQUE INDEX IF NOT EXISTS platform.ux_operation_outbox_idempotency_key ON operation_outbox(idempotency_key)",
			"CREATE INDEX IF NOT EXISTS platform.ix_operation_outbox_claim ON operation_outbox(status, next_attempt_at, created_at)",
		}
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
