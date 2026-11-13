package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/sftpgateway"
)

func TestNewFromConfigAppliesSFTPDefaults(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	landing := filepath.ToSlash(filepath.Join(root, "landing"))
	cfgYAML := []byte("filesystem:\n  sftp:\n    enabled: true\n    landing_zone: \"" + landing + "\"\n")
	require.NoError(t, os.WriteFile(cfgPath, cfgYAML, 0o600))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	require.True(t, cfg.Filesystem.SFTP.Enabled)
	require.Equal(t, ":2022", cfg.Filesystem.SFTP.Listen)
	require.Equal(t, 15*time.Minute, cfg.Filesystem.SFTP.IdleTimeout)

	server, err := sftpgateway.NewFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, server)
}

func TestExampleConfigIncludesSFTPBlock(t *testing.T) {
	data, err := os.ReadFile("../../config.example.yaml")
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(data, &parsed))
	fs, ok := parsed["filesystem"].(map[string]any)
	require.True(t, ok)
	sftpBlock, ok := fs["sftp"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, ":2022", sftpBlock["listen"])
	require.Equal(t, true, sftpBlock["enabled"])
}
