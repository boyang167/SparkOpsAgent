package persistence

import (
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS platform").Error; err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(&EventOutbox{}, &EventInbox{}, &OperationOutbox{}); err != nil {
		return err
	}
	return createIndexes(db)
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
