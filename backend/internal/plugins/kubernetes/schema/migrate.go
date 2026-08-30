package schema

import (
	"time"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS plugin_kubernetes").Error; err != nil {
			return err
		}
		return db.AutoMigrate(&ClusterSettings{}, &Record{}, &Diagnosis{})
	}
	if err := db.AutoMigrate(&ClusterSettings{}, &Diagnosis{}, &sqliteRecord{}); err != nil {
		return err
	}
	return db.Exec("CREATE INDEX IF NOT EXISTS plugin_kubernetes.idx_records_workspace_started_at ON records(workspace_id, started_at DESC)").Error
}

// sqliteRecord mirrors Record without gorm index tags; SQLite mishandles dotted
// table names when AutoMigrate creates indexes from struct tags.
type sqliteRecord struct {
	ID          string `gorm:"primaryKey;size:36"`
	WorkspaceID string `gorm:"size:36"`
	ClusterID   string `gorm:"size:36"`
	Kind        string
	StartedAt   time.Time
	EndedAt     *time.Time
	Blob        []byte
	ObjectPath  string
}

func (sqliteRecord) TableName() string { return "plugin_kubernetes.records" }
