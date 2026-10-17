package persistence

import (
	"fmt"
	"strings"
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

	requireSQLiteColumns(t, db, "event_outbox", map[string]expectedSQLiteColumn{
		"id":                {Type: "uuid", NotNull: true},
		"event_type":        {Type: "text", NotNull: true},
		"event_version":     {Type: "integer", NotNull: true},
		"aggregate_type":    {Type: "text", NotNull: true, Default: defaultValue("")},
		"aggregate_id":      {Type: "text", NotNull: true, Default: defaultValue("")},
		"aggregate_version": {Type: "integer"},
		"payload":           {Type: "jsonb", NotNull: true},
		"occurred_at":       {Type: "datetime", NotNull: true},
		"correlation_id":    {Type: "uuid"},
		"causation_id":      {Type: "uuid"},
		"status":            {Type: "text", NotNull: true, Default: defaultValue("pending")},
		"attempts":          {Type: "integer", NotNull: true, Default: defaultValue("0")},
		"next_attempt_at":   {Type: "datetime", NotNull: true, Default: defaultValue("CURRENT_TIMESTAMP")},
		"locked_at":         {Type: "datetime"},
		"locked_by":         {Type: "text", NotNull: true, Default: defaultValue("")},
		"published_at":      {Type: "datetime"},
		"last_error":        {Type: "text", NotNull: true, Default: defaultValue("")},
		"created_at":        {Type: "datetime", NotNull: true, Default: defaultValue("CURRENT_TIMESTAMP")},
	})
	requireSQLiteColumns(t, db, "event_inbox", map[string]expectedSQLiteColumn{
		"event_id":     {Type: "uuid", NotNull: true},
		"subscriber":   {Type: "text", NotNull: true},
		"event_type":   {Type: "text", NotNull: true},
		"processed_at": {Type: "datetime", NotNull: true, Default: defaultValue("CURRENT_TIMESTAMP")},
		"created_at":   {Type: "datetime", NotNull: true, Default: defaultValue("CURRENT_TIMESTAMP")},
	})
	requireSQLiteColumns(t, db, "operation_outbox", map[string]expectedSQLiteColumn{
		"id":              {Type: "uuid", NotNull: true},
		"event_id":        {Type: "uuid", NotNull: true},
		"subscriber":      {Type: "text", NotNull: true},
		"operation_type":  {Type: "text", NotNull: true},
		"idempotency_key": {Type: "text", NotNull: true},
		"payload":         {Type: "jsonb", NotNull: true},
		"status":          {Type: "text", NotNull: true, Default: defaultValue("pending")},
		"attempts":        {Type: "integer", NotNull: true, Default: defaultValue("0")},
		"next_attempt_at": {Type: "datetime", NotNull: true, Default: defaultValue("CURRENT_TIMESTAMP")},
		"locked_at":       {Type: "datetime"},
		"locked_by":       {Type: "text", NotNull: true, Default: defaultValue("")},
		"completed_at":    {Type: "datetime"},
		"last_error":      {Type: "text", NotNull: true, Default: defaultValue("")},
		"created_at":      {Type: "datetime", NotNull: true, Default: defaultValue("CURRENT_TIMESTAMP")},
	})
	requireGORMFields(t, db, &EventOutbox{}, map[string]expectedGORMField{
		"EventType":     {DataType: "string", Size: 160},
		"AggregateType": {DataType: "string", Size: 120},
		"AggregateID":   {DataType: "string", Size: 160},
		"Status":        {DataType: "string", Size: 16},
		"LockedBy":      {DataType: "string", Size: 160},
	})
	requireGORMFields(t, db, &EventInbox{}, map[string]expectedGORMField{
		"Subscriber": {DataType: "string", Size: 200},
		"EventType":  {DataType: "string", Size: 160},
	})
	requireGORMFields(t, db, &OperationOutbox{}, map[string]expectedGORMField{
		"Subscriber":     {DataType: "string", Size: 200},
		"OperationType":  {DataType: "string", Size: 160},
		"IdempotencyKey": {DataType: "string", Size: 240},
		"Status":         {DataType: "string", Size: 16},
		"LockedBy":       {DataType: "string", Size: 160},
	})

	require.Equal(t, map[string]int{"event_id": 1, "subscriber": 2}, sqlitePrimaryKey(t, db, "event_inbox"))
	requireSQLiteIndex(t, db, "event_outbox", "ix_event_outbox_claim", false,
		"status", "next_attempt_at", "created_at",
	)
	requireSQLiteIndex(t, db, "operation_outbox", "ux_operation_outbox_idempotency_key", true,
		"idempotency_key",
	)
	requireSQLiteIndex(t, db, "operation_outbox", "ix_operation_outbox_claim", false,
		"status", "next_attempt_at", "created_at",
	)
}

func TestAutoMigrateRejectsInvalidOutboxStatuses(t *testing.T) {
	db := openAttachedPlatformSQLite(t, sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"))
	require.NoError(t, AutoMigrate(db))

	eventErr := db.Exec(`
		INSERT INTO platform.event_outbox
			(id, event_type, event_version, payload, occurred_at, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "event-1", "test.event", 1, `{}`, "2026-10-17T00:00:00Z", "invalid").Error
	require.ErrorContains(t, eventErr, "event_outbox_status_check")

	operationErr := db.Exec(`
		INSERT INTO platform.operation_outbox
			(id, event_id, subscriber, operation_type, idempotency_key, payload, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "operation-1", "event-1", "test-subscriber", "test.operation", "key-1", `{}`, "invalid").Error
	require.ErrorContains(t, operationErr, "operation_outbox_status_check")
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
	require.NoError(t, db.Exec("INSERT INTO public.schema_migrations (version, dirty) VALUES (72, false)").Error)

	require.NoError(t, AutoMigrate(db))

	var count int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM platform.sqlite_master WHERE type = 'table' AND name IN ?",
		[]string{"event_outbox", "event_inbox", "operation_outbox"},
	).Scan(&count).Error)
	require.Zero(t, count)
}

func TestAutoMigrateDoesNotCreateTransactionalTablesOnFreshPostgres(t *testing.T) {
	dialector := postgresNamedSQLite{Dialector: sqlite.Open("file:" + t.Name() + "?mode=memory&cache=shared")}
	db := openAttachedPlatformSQLite(t, dialector)

	require.NoError(t, AutoMigrate(db))

	var count int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM platform.sqlite_master WHERE type = 'table' AND name IN ?",
		[]string{"event_outbox", "event_inbox", "operation_outbox"},
	).Scan(&count).Error)
	require.Zero(t, count)
}

func TestValidateTransactionalEventSchema(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T, *gorm.DB)
		wantErr string
	}{
		{
			name: "fresh database",
			setup: func(t *testing.T, db *gorm.DB) {
				attachMigrationSchemas(t, db)
				require.NoError(t, db.Exec(
					"CREATE TABLE public.schema_migrations (version INTEGER, dirty BOOLEAN)",
				).Error)
			},
			wantErr: "found 0",
		},
		{
			name: "migration query error",
			setup: func(t *testing.T, db *gorm.DB) {
				attachMigrationSchemas(t, db)
			},
			wantErr: "query public.schema_migrations",
		},
		{
			name: "old migration",
			setup: func(t *testing.T, db *gorm.DB) {
				attachMigrationSchemas(t, db)
				createMigrationState(t, db, 71, false)
			},
			wantErr: "version 72",
		},
		{
			name: "dirty migration",
			setup: func(t *testing.T, db *gorm.DB) {
				attachMigrationSchemas(t, db)
				createMigrationState(t, db, 72, true)
			},
			wantErr: "dirty",
		},
		{
			name: "current migration missing tables",
			setup: func(t *testing.T, db *gorm.DB) {
				attachMigrationSchemas(t, db)
				createMigrationState(t, db, 72, false)
			},
			wantErr: "platform tables",
		},
		{
			name: "current migration",
			setup: func(t *testing.T, db *gorm.DB) {
				attachMigrationSchemas(t, db)
				createMigrationState(t, db, 72, false)
				for _, table := range []string{"event_outbox", "event_inbox", "operation_outbox"} {
					require.NoError(t, db.Exec(
						"INSERT INTO information_schema.tables (table_schema, table_name) VALUES ('platform', ?)",
						table,
					).Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialector := postgresNamedSQLite{
				Dialector: sqlite.Open("file:" + strings.ReplaceAll(t.Name(), "/", "-") + "?mode=memory&cache=shared"),
			}
			db := openAttachedPlatformSQLite(t, dialector)
			tt.setup(t, db)

			err := ValidateTransactionalEventSchema(db)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateTransactionalEventSchemaAllowsSQLiteUnitTests(t *testing.T) {
	db := openAttachedPlatformSQLite(t, sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"))
	require.NoError(t, ValidateTransactionalEventSchema(db))
}

type migrationState struct {
	version int
	dirty   bool
}

func TestValidateTransactionalEventSchemaRejectsMultipleMigrationRowsRegardlessOfOrder(t *testing.T) {
	tests := []struct {
		name string
		rows []migrationState
	}{
		{
			name: "clean then dirty",
			rows: []migrationState{{version: 72}, {version: 72, dirty: true}},
		},
		{
			name: "dirty then clean",
			rows: []migrationState{{version: 72, dirty: true}, {version: 72}},
		},
		{
			name: "current then old",
			rows: []migrationState{{version: 72}, {version: 71}},
		},
		{
			name: "old then current",
			rows: []migrationState{{version: 71}, {version: 72}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialector := postgresNamedSQLite{
				Dialector: sqlite.Open("file:" + strings.ReplaceAll(t.Name(), "/", "-") + "?mode=memory&cache=shared"),
			}
			db := openAttachedPlatformSQLite(t, dialector)
			attachMigrationSchemas(t, db)
			require.NoError(t, db.Exec(
				"CREATE TABLE public.schema_migrations (version INTEGER, dirty BOOLEAN)",
			).Error)
			for _, row := range tt.rows {
				require.NoError(t, db.Exec(
					"INSERT INTO public.schema_migrations (version, dirty) VALUES (?, ?)",
					row.version,
					row.dirty,
				).Error)
			}

			err := ValidateTransactionalEventSchema(db)
			require.ErrorContains(t, err, "exactly one row")
			require.ErrorContains(t, err, "found 2")
			require.ErrorContains(t, err, "make migrate-up")
		})
	}
}

func attachMigrationSchemas(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS information_schema").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS public").Error)
	require.NoError(t, db.Exec(
		"CREATE TABLE information_schema.tables (table_schema TEXT, table_name TEXT)",
	).Error)
}

func createMigrationState(t *testing.T, db *gorm.DB, version int, dirty bool) {
	t.Helper()
	require.NoError(t, db.Exec(
		"CREATE TABLE public.schema_migrations (version INTEGER, dirty BOOLEAN)",
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO public.schema_migrations (version, dirty) VALUES (?, ?)",
		version,
		dirty,
	).Error)
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

type expectedSQLiteColumn struct {
	Type    string
	NotNull bool
	Default *string
}

func defaultValue(value string) *string {
	return &value
}

func requireSQLiteColumns(t *testing.T, db *gorm.DB, table string, expected map[string]expectedSQLiteColumn) {
	t.Helper()
	type column struct {
		Name         string
		Type         string
		NotNull      int     `gorm:"column:notnull"`
		DefaultValue *string `gorm:"column:dflt_value"`
	}
	var columns []column
	require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA platform.table_info(%q)", table)).Scan(&columns).Error)
	require.Len(t, columns, len(expected))
	for _, column := range columns {
		want, ok := expected[column.Name]
		require.Truef(t, ok, "unexpected column %s.%s", table, column.Name)
		require.Equal(t, want.Type, strings.ToLower(column.Type), "%s.%s type", table, column.Name)
		require.Equal(t, want.NotNull, column.NotNull == 1, "%s.%s NOT NULL", table, column.Name)
		require.Equal(t, normalizedDefault(want.Default), normalizedDefault(column.DefaultValue), "%s.%s default", table, column.Name)
	}
}

func normalizedDefault(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	normalized = strings.Trim(normalized, "'\"")
	return &normalized
}

type expectedGORMField struct {
	DataType string
	Size     int
}

func requireGORMFields(t *testing.T, db *gorm.DB, model any, expected map[string]expectedGORMField) {
	t.Helper()
	statement := &gorm.Statement{DB: db}
	require.NoError(t, statement.Parse(model))
	for name, want := range expected {
		field := statement.Schema.LookUpField(name)
		require.NotNilf(t, field, "missing GORM field %s", name)
		require.Equal(t, want.DataType, string(field.DataType), "%s data type", name)
		require.Equal(t, want.Size, field.Size, "%s size", name)
	}
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
