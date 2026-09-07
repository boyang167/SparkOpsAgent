package service

import (
	"testing"

	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/stretchr/testify/require"
)

func TestExtractManifestResourceNames_SuffixConvention(t *testing.T) {
	manifests := []runtimeport.Manifest{
		{YAML: `apiVersion: v1
kind: Service
metadata:
  name: orders-service
  namespace: team-a`},
		{YAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders-deployment
  namespace: team-a`},
	}

	deploymentName, serviceName, err := extractManifestResourceNames(manifests)
	require.NoError(t, err)
	require.Equal(t, "orders-deployment", deploymentName)
	require.Equal(t, "orders-service", serviceName)
}

func TestExtractManifestResourceNames_ProjectNameConvention(t *testing.T) {
	manifests := []runtimeport.Manifest{
		{YAML: `apiVersion: v1
kind: Service
metadata:
  name: test-repo
  namespace: test-workspace`},
		{YAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-repo
  namespace: test-workspace`},
	}

	deploymentName, serviceName, err := extractManifestResourceNames(manifests)
	require.NoError(t, err)
	require.Equal(t, "test-repo", deploymentName)
	require.Equal(t, "test-repo", serviceName)
}
