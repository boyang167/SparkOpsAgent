package bootstrap

import (
	"os"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateMessagingBootstrapRejectsOldAMQPSchema(t *testing.T) {
	db, err := gorm.Open(bootstrapPostgresNamedSQLite{
		Dialector: sqlite.Open("file:" + t.Name() + "?mode=memory&cache=shared"),
	}, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS information_schema").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS public").Error)
	require.NoError(t, db.Exec(
		"CREATE TABLE information_schema.tables (table_schema TEXT, table_name TEXT)",
	).Error)
	require.NoError(t, db.Exec(
		"CREATE TABLE public.schema_migrations (version INTEGER, dirty BOOLEAN)",
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO public.schema_migrations (version, dirty) VALUES (69, false)",
	).Error)

	err = validateMessagingBootstrap(
		&config.Config{Messaging: config.MessagingConfig{Driver: "amqp"}},
		db,
	)
	require.ErrorContains(t, err, "version 74")
	require.ErrorContains(t, err, "make migrate-up")
}

func TestRunChecksMessagingSchemaBeforeFactoryAndHTTPServer(t *testing.T) {
	source, err := os.ReadFile("app.go")
	require.NoError(t, err)
	text := string(source)

	schemaGate := strings.Index(text, "validateMessagingBootstrap(cfg, db)")
	factory := strings.Index(text, "messaging.NewFromConfig(")
	httpServer := strings.Index(text, "server := &http.Server{")
	require.NotEqual(t, -1, schemaGate)
	require.NotEqual(t, -1, factory)
	require.NotEqual(t, -1, httpServer)
	require.Less(t, schemaGate, factory)
	require.Less(t, factory, httpServer)
}

type bootstrapPostgresNamedSQLite struct {
	gorm.Dialector
}

func (bootstrapPostgresNamedSQLite) Name() string { return "postgres" }
