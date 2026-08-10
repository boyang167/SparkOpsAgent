package k8s_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
	"github.com/stretchr/testify/require"
)

func TestNewFactory_empty_config(t *testing.T) {
	f, err := k8s.NewFactory(k8s.Config{})
	require.NoError(t, err)
	require.Empty(t, f.Names())
}

func TestFactory_Get_errors_without_clusters(t *testing.T) {
	f, err := k8s.NewFactory(k8s.Config{})
	require.NoError(t, err)

	_, err = f.Default()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no kubernetes cluster configured")
}
