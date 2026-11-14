package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/sparkops/sparkops-agent/backend/internal/filesystemgateway"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func TestNewEngineFromConfigAppliesTransferGatewayDefaults(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	storageRoot := filepath.ToSlash(filepath.Join(root, "data"))
	cfgYAML := []byte("jwt:\n  secret: gateway-test-secret\nfilesystem:\n  transfer_gateway:\n    storage_root: \"" + storageRoot + "\"\n")
	require.NoError(t, os.WriteFile(cfgPath, cfgYAML, 0o600))

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	require.Equal(t, ":8082", cfg.Filesystem.TransferGateway.Listen)
	require.Equal(t, "sparkops-file-transfer", cfg.Filesystem.TransferGateway.CapabilityAudience)
	require.Equal(t, int64(21474836480), cfg.Filesystem.TransferGateway.MaxFileBytes)
	require.Equal(t, "gateway-test-secret", cfg.Filesystem.TransferGateway.CapabilitySecret)

	engine, err := filesystemgateway.NewEngineFromConfig(cfg)
	require.NoError(t, err)
	defer engine.Close()
	require.NotNil(t, engine)
}

func TestExampleConfigIncludesTransferGatewayBlock(t *testing.T) {
	data, err := os.ReadFile("../../config.example.yaml")
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(data, &parsed))
	fs, ok := parsed["filesystem"].(map[string]any)
	require.True(t, ok)
	tg, ok := fs["transfer_gateway"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, ":8082", tg["listen"])
}
