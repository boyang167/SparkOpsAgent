package config_test

import (
	"testing"
	"time"

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
		Delivery: config.DeliveryConfig{Driver: "mock"},
		Runtime:  config.RuntimeConfig{Driver: "mock"},
		Deploy: config.DeployConfig{
			JenkinsTimeout: time.Minute, RolloutTimeout: time.Minute, ReconcileInterval: time.Second,
		},
	}
	require.NoError(t, cfg.Validate())
}

func TestValidate_requiresJenkinsWhenDriverIsJenkins(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{DSN: "postgres://localhost/sparkops"},
		JWT:      config.JWTConfig{Secret: "long-enough-secret"},
		Delivery: config.DeliveryConfig{Driver: "jenkins"},
		Runtime:  config.RuntimeConfig{Driver: "k8s"},
		Deploy: config.DeployConfig{
			JenkinsTimeout: time.Minute, RolloutTimeout: time.Minute, ReconcileInterval: time.Second,
		},
	}
	require.NoError(t, cfg.Validate())
}
