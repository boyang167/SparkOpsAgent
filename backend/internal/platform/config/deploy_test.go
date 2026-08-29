package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeployTimeoutDefaultsAndEnvironmentOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))
	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, 30*time.Minute, cfg.Deploy.JenkinsTimeout)
	require.Equal(t, 10*time.Minute, cfg.Deploy.RolloutTimeout)
	require.Equal(t, 30*time.Second, cfg.Deploy.ReconcileInterval)

	t.Setenv("DEPLOY_JENKINS_TIMEOUT", "45m")
	t.Setenv("DEPLOY_ROLLOUT_TIMEOUT", "15m")
	t.Setenv("DEPLOY_RECONCILE_INTERVAL", "5s")
	cfg, err = Load(path)
	require.NoError(t, err)
	require.Equal(t, 45*time.Minute, cfg.Deploy.JenkinsTimeout)
	require.Equal(t, 15*time.Minute, cfg.Deploy.RolloutTimeout)
	require.Equal(t, 5*time.Second, cfg.Deploy.ReconcileInterval)
}

func TestDeployTimeoutRejectsInvalidEnvironmentDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))
	t.Setenv("DEPLOY_JENKINS_TIMEOUT", "never")
	_, err := Load(path)
	require.Error(t, err)
}

func TestValidateRejectsNonPositiveDeployDurations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
		field  string
	}{
		{"jenkins", func(c *Config) { c.Deploy.JenkinsTimeout = 0 }, "deploy.jenkins_timeout"},
		{"rollout", func(c *Config) { c.Deploy.RolloutTimeout = -time.Second }, "deploy.rollout_timeout"},
		{"reconcile", func(c *Config) { c.Deploy.ReconcileInterval = 0 }, "deploy.reconcile_interval"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Database: DatabaseConfig{DSN: "postgres://test"},
				JWT:      JWTConfig{Secret: "0123456789abcdef"},
				Deploy: DeployConfig{
					JenkinsTimeout: time.Minute, RolloutTimeout: time.Minute, ReconcileInterval: time.Second,
				},
			}
			tc.mutate(cfg)
			require.ErrorContains(t, cfg.Validate(), tc.field+" must be greater than zero")
		})
	}
}

func TestDeployTimeoutsLoadFromYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("deploy:\n  jenkins_timeout: 35m\n  rollout_timeout: 12m\n"), 0o600))
	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, 35*time.Minute, cfg.Deploy.JenkinsTimeout)
	require.Equal(t, 12*time.Minute, cfg.Deploy.RolloutTimeout)
}
