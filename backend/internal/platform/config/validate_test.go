package config_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestValidate_requiresDatabaseDSN(t *testing.T) {
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "long-enough-secret"}}
	require.Error(t, cfg.Validate())
}

func TestValidate_acceptsMinimalDevConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{DSN: "postgres://localhost/sparkops"},
		JWT:      config.JWTConfig{Secret: "long-enough-secret"},
	}
	require.NoError(t, cfg.Validate())
}
