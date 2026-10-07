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
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS sprint_process").Error; err != nil {
			return err
		}
		for _, table := range []string{"sprint_processes", "workspace_sprint_process_templates"} {
			var exists int64
			if err := db.Raw(`
				SELECT COUNT(*) FROM information_schema.tables
				WHERE table_schema = 'product' AND table_name = ?
			`, table).Scan(&exists).Error; err != nil {
				return err
			}
			if exists == 0 {
				continue
			}
			if err := db.Exec("ALTER TABLE product." + table + " SET SCHEMA sprint_process").Error; err != nil {
				return err
			}
		}
	}
	return db.AutoMigrate(&sprintProcessModel{}, &workspaceSprintTemplateModel{})
}
