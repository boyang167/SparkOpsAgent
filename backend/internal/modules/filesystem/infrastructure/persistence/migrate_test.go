package persistence

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type postgresNamedSQLite struct {
	gorm.Dialector
}

func (postgresNamedSQLite) Name() string { return "postgres" }

func TestAutoMigrateSkipsAfterSQLMigrationsApplied(t *testing.T) {
	db, err := gorm.Open(postgresNamedSQLite{
		Dialector: sqlite.Open("file:" + t.Name() + "?mode=memory&cache=shared"),
	}, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS information_schema").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS public").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, db.Exec("CREATE TABLE information_schema.tables (table_schema TEXT, table_name TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO information_schema.tables (table_schema, table_name) VALUES ('public', 'schema_migrations')").Error)
	require.NoError(t, db.Exec("CREATE TABLE public.schema_migrations (version INTEGER, dirty BOOLEAN)").Error)
	require.NoError(t, db.Exec("INSERT INTO public.schema_migrations (version, dirty) VALUES (77, false)").Error)

	require.NoError(t, AutoMigrate(db))

	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM filesystem.sqlite_master WHERE type = 'table'").Scan(&count).Error)
	require.Zero(t, count)
}
