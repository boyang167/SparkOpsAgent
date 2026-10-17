package persistence

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAutoMigrateCreatesTransactionalEventSchema(t *testing.T) {
	db := openAttachedPlatformSQLite(t, sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"))

	require.NoError(t, AutoMigrate(db))

	for _, table := range []string{"event_outbox", "event_inbox", "operation_outbox"} {
		requireSQLiteObject(t, db, "table", table)
	}

	requireSQLiteColumns(t, db, "event_outbox",
		"id", "event_type", "event_version", "payload", "status",
		"next_attempt_at", "locked_at", "locked_by", "created_at",
	)
	requireSQLiteColumns(t, db, "event_inbox",
		"event_id", "subscriber", "event_type", "processed_at", "created_at",
	)
	requireSQLiteColumns(t, db, "operation_outbox",
		"id", "event_id", "subscriber", "operation_type", "idempotency_key",
		"payload", "status", "next_attempt_at", "created_at",
	)

	require.Equal(t, map[string]int{"event_id": 1, "subscriber": 2}, sqlitePrimaryKey(t, db, "event_inbox"))
	requireSQLiteIndex(t, db, "event_outbox", "ix_event_outbox_claim", false,
		"status", "next_attempt_at", "created_at",
	)
	requireSQLiteIndex(t, db, "operation_outbox", "ux_operation_outbox_idempotency_key", true,
		"idempotency_key",
	)
}

func TestAutoMigrateSkipsWhenSQLMigrationsApplied(t *testing.T) {
	dialector := postgresNamedSQLite{Dialector: sqlite.Open("file:" + t.Name() + "?mode=memory&cache=shared")}
	db := openAttachedPlatformSQLite(t, dialector)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS information_schema").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS public").Error)
	require.NoError(t, db.Exec("CREATE TABLE information_schema.tables (table_schema TEXT, table_name TEXT)").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO information_schema.tables (table_schema, table_name) VALUES ('public', 'schema_migrations')",
	).Error)
	require.NoError(t, db.Exec("CREATE TABLE public.schema_migrations (version INTEGER, dirty BOOLEAN)").Error)
	require.NoError(t, db.Exec("INSERT INTO public.schema_migrations (version, dirty) VALUES (70, false)").Error)

	require.NoError(t, AutoMigrate(db))

	var count int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM platform.sqlite_master WHERE type = 'table' AND name IN ?",
		[]string{"event_outbox", "event_inbox", "operation_outbox"},
	).Scan(&count).Error)
	require.Zero(t, count)
}

type postgresNamedSQLite struct {
	gorm.Dialector
}

func (postgresNamedSQLite) Name() string { return "postgres" }

func openAttachedPlatformSQLite(t *testing.T, dialector gorm.Dialector) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	return db
}

func requireSQLiteObject(t *testing.T, db *gorm.DB, objectType, name string) {
	t.Helper()
	var count int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM platform.sqlite_master WHERE type = ? AND name = ?",
		objectType, name,
	).Scan(&count).Error)
	require.Equal(t, int64(1), count)
}

func requireSQLiteColumns(t *testing.T, db *gorm.DB, table string, expected ...string) {
	t.Helper()
	type column struct {
		Name string
	}
	var columns []column
	require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA platform.table_info(%q)", table)).Scan(&columns).Error)
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name)
	}
	require.Subset(t, names, expected)
}

func sqlitePrimaryKey(t *testing.T, db *gorm.DB, table string) map[string]int {
	t.Helper()
	type column struct {
		Name string
		PK   int `gorm:"column:pk"`
	}
	var columns []column
	require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA platform.table_info(%q)", table)).Scan(&columns).Error)
	key := make(map[string]int)
	for _, column := range columns {
		if column.PK > 0 {
			key[column.Name] = column.PK
		}
	}
	return key
}

func requireSQLiteIndex(t *testing.T, db *gorm.DB, table, index string, unique bool, expectedColumns ...string) {
	t.Helper()
	type indexRow struct {
		Name   string
		Unique int
	}
	var indexes []indexRow
	require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA platform.index_list(%q)", table)).Scan(&indexes).Error)
	for _, row := range indexes {
		if row.Name != index {
			continue
		}
		if unique {
			require.Equal(t, 1, row.Unique)
		} else {
			require.Zero(t, row.Unique)
		}

		type indexColumn struct {
			Name string
		}
		var columns []indexColumn
		require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA platform.index_info(%q)", index)).Scan(&columns).Error)
		names := make([]string, 0, len(columns))
		for _, column := range columns {
			names = append(names, column.Name)
		}
		require.Equal(t, expectedColumns, names)
		return
	}
	require.Failf(t, "missing index", "%s on %s", index, table)
}
