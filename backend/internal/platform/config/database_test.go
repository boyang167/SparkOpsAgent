package config_test

import (
	"os"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestDatabaseConfig_EffectiveDSN_encodesSpecialCharsInPassword(t *testing.T) {
	dsn, err := config.DatabaseConfig{
		Host:     "192.168.5.24",
		Port:     5432,
		User:     "admin",
		Password: "admin@2025",
		Name:     "sparkops_agent",
		SSLMode:  "disable",
	}.EffectiveDSN()
	require.NoError(t, err)
	require.Contains(t, dsn, "admin:admin%402025@192.168.5.24:5432/sparkops_agent")
}

func TestDatabaseConfig_EffectiveDSN_prefersExplicitDSN(t *testing.T) {
	raw := "postgres://u:p@localhost/db?sslmode=disable"
	dsn, err := config.DatabaseConfig{DSN: raw, Host: "ignored"}.EffectiveDSN()
	require.NoError(t, err)
	require.Equal(t, raw, dsn)
}

func TestLoad_buildsDSNFromStructuredFields(t *testing.T) {
	path := writeTempConfig(t, `
server:
  addr: ":8080"
database:
  host: db.internal
  port: 5432
  user: sparkops
  password: "p@ss:wrd"
  name: sparkops
jwt:
  secret: long-enough-secret-key
`)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Contains(t, cfg.Database.DSN, "db.internal:5432/sparkops")
	require.Contains(t, cfg.Database.DSN, "sparkops:")
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "cfg-*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.WriteString(body)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}
