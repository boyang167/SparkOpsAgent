package config_test

import (
	"os"
	"path/filepath"
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

func TestMessagingDefaultsAndEnvironmentOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
messaging:
  driver: inmemory
  amqp:
    event_exchange: yaml.events
  outbox:
    poll_interval: 2s
    batch_size: 25
    lock_timeout: 2m
`), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, "yaml.events", cfg.Messaging.AMQP.EventExchange)
	require.Equal(t, 2*time.Second, cfg.Messaging.Outbox.PollInterval)
	require.Equal(t, 25, cfg.Messaging.Outbox.BatchSize)
	require.Equal(t, 2*time.Minute, cfg.Messaging.Outbox.LockTimeout)

	t.Setenv("AMQP_EVENT_EXCHANGE", "env.events")
	t.Setenv("OUTBOX_POLL_INTERVAL", "3s")
	t.Setenv("OUTBOX_BATCH_SIZE", "50")
	t.Setenv("OUTBOX_LOCK_TIMEOUT", "90s")
	cfg, err = config.Load(path)
	require.NoError(t, err)
	require.Equal(t, "env.events", cfg.Messaging.AMQP.EventExchange)
	require.Equal(t, 3*time.Second, cfg.Messaging.Outbox.PollInterval)
	require.Equal(t, 50, cfg.Messaging.Outbox.BatchSize)
	require.Equal(t, 90*time.Second, cfg.Messaging.Outbox.LockTimeout)
}

func TestMessagingDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, "sparkops.events", cfg.Messaging.AMQP.EventExchange)
	require.Equal(t, time.Second, cfg.Messaging.Outbox.PollInterval)
	require.Equal(t, 100, cfg.Messaging.Outbox.BatchSize)
	require.Equal(t, time.Minute, cfg.Messaging.Outbox.LockTimeout)
}

func TestMessagingEnvironmentOverridesRejectInvalidValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))

	for _, tc := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "poll interval", key: "OUTBOX_POLL_INTERVAL", value: "never"},
		{name: "batch size", key: "OUTBOX_BATCH_SIZE", value: "many"},
		{name: "lock timeout", key: "OUTBOX_LOCK_TIMEOUT", value: "never"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			_, err := config.Load(path)
			require.ErrorContains(t, err, tc.key)
		})
	}
}

func TestValidateProductionMessagingRequirements(t *testing.T) {
	t.Setenv("APP_ENV", config.EnvProduction)
	base := validConfig()
	base.JWT.Secret = "production-secret-value"

	t.Run("rejects inmemory", func(t *testing.T) {
		cfg := *base
		cfg.Messaging.Driver = "inmemory"
		require.ErrorContains(t, cfg.Validate(), "must not be inmemory")
	})
	t.Run("rejects missing AMQP URL", func(t *testing.T) {
		cfg := *base
		cfg.Messaging.Driver = "amqp"
		cfg.Messaging.AMQP.URL = ""
		require.ErrorContains(t, cfg.Validate(), "messaging.amqp.url")
	})
	t.Run("rejects missing database", func(t *testing.T) {
		cfg := *base
		cfg.Database.DSN = ""
		cfg.Messaging.Driver = "amqp"
		cfg.Messaging.AMQP.URL = "amqp://localhost"
		require.ErrorContains(t, cfg.Validate(), "database.dsn")
	})
}

func TestValidateMessagingOutboxValues(t *testing.T) {
	cfg := validConfig()
	cfg.Messaging.Driver = "amqp"
	cfg.Messaging.AMQP.URL = "amqp://localhost"
	cfg.Messaging.AMQP.EventExchange = "sparkops.events"
	cfg.Messaging.Outbox = config.OutboxConfig{
		PollInterval: time.Second,
		BatchSize:    100,
		LockTimeout:  time.Minute,
	}

	require.NoError(t, cfg.Validate())
	cfg.Messaging.Outbox.BatchSize = 0
	require.ErrorContains(t, cfg.Validate(), "messaging.outbox.batch_size")
}

func validConfig() *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{DSN: "postgres://localhost/sparkops"},
		JWT:      config.JWTConfig{Secret: "long-enough-secret"},
		Delivery: config.DeliveryConfig{Driver: "jenkins"},
		Runtime:  config.RuntimeConfig{Driver: "k8s"},
		Deploy: config.DeployConfig{
			JenkinsTimeout:    time.Minute,
			RolloutTimeout:    time.Minute,
			ReconcileInterval: time.Second,
		},
		Messaging: config.MessagingConfig{Driver: "inmemory"},
	}
}
