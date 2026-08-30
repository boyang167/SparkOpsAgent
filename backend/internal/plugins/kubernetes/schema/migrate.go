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
		if !db.Migrator().HasTable(&ClusterSettings{}) {
			if err := db.AutoMigrate(&ClusterSettings{}); err != nil {
				return err
			}
		} else if err := migratePostgresClusterSettings(db); err != nil {
			return err
		}
		return db.AutoMigrate(&Record{}, &Diagnosis{})
	}
	if err := db.AutoMigrate(&ClusterSettings{}, &Diagnosis{}, &sqliteRecord{}); err != nil {
		return err
	}
	return db.Exec("CREATE INDEX IF NOT EXISTS plugin_kubernetes.idx_records_workspace_started_at ON records(workspace_id, started_at DESC)").Error
}

func migratePostgresClusterSettings(db *gorm.DB) error {
	statements := []string{
		`ALTER TABLE plugin_kubernetes.cluster_settings ADD COLUMN IF NOT EXISTS workspace_id varchar(36)`,
		`DO $$
		DECLARE c record;
		BEGIN
		  FOR c IN
		    SELECT conname FROM pg_constraint
		    WHERE conrelid = 'plugin_kubernetes.cluster_settings'::regclass
		      AND contype IN ('p','u')
		      AND pg_get_constraintdef(oid) ~ '[(]cluster_id[)]'
		  LOOP
		    EXECUTE format('ALTER TABLE plugin_kubernetes.cluster_settings DROP CONSTRAINT %I', c.conname);
		  END LOOP;
		END $$`,
		`DO $$
		DECLARE i record;
		BEGIN
		  FOR i IN
		    SELECT indexname FROM pg_indexes
		    WHERE schemaname = 'plugin_kubernetes' AND tablename = 'cluster_settings'
		      AND indexdef ~ 'UNIQUE .*[(]cluster_id[)]'
		  LOOP
		    EXECUTE format('DROP INDEX IF EXISTS plugin_kubernetes.%I', i.indexname);
		  END LOOP;
		END $$`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_cluster_settings_workspace_cluster
		 ON plugin_kubernetes.cluster_settings(workspace_id, cluster_id)
		 WHERE workspace_id IS NOT NULL`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
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
