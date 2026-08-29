package service

import (
	"context"
	"strings"
	"testing"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/stretchr/testify/require"
)

func TestRendererUsesRunRepositoryAndTagWithoutZeroValueYAML(t *testing.T) {
	project := deployport.ProjectDeploymentContext{
		Name: "orders", Namespace: "team-a", ImageRepository: "registry.example/team-a/orders",
		ServiceYAML: `apiVersion: v1
kind: Service
metadata:
  name: orders-service
  namespace: team-a
spec:
  selector:
    app: orders
  ports:
    - port: 80
`,
		DeploymentYAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders-deployment
  namespace: team-a
spec:
  selector:
    matchLabels:
      app: orders
  template:
    metadata:
      labels:
        app: orders
    spec:
      containers:
        - name: orders
          image: stale.example/orders:old
`,
	}

	manifests, err := NewManifestRenderer().Render(context.Background(), project, "run-123")

	require.NoError(t, err)
	require.Len(t, manifests, 2)
	joined := manifests[0].YAML + manifests[1].YAML
	require.Contains(t, joined, "registry.example/team-a/orders:run-123")
	require.NotContains(t, joined, "stale.example/orders:old")
	for _, forbidden := range []string{"name: \"\"", "namespace: \"\"", "image: \":run-123\""} {
		require.False(t, strings.Contains(joined, forbidden), joined)
	}
}

func TestRendererReusesWorkspaceDefaultsWhenStoredArtifactsAreEmpty(t *testing.T) {
	manifests, err := NewManifestRenderer().Render(context.Background(), deployport.ProjectDeploymentContext{
		Name: "orders", Namespace: "team-a", ImageRepository: "registry/orders",
	}, "run-123")
	require.NoError(t, err)
	require.Len(t, manifests, 2)
	joined := manifests[0].YAML + manifests[1].YAML
	require.Contains(t, joined, "kind: Service")
	require.Contains(t, joined, "kind: Deployment")
	require.Contains(t, joined, "registry/orders:run-123")
}
