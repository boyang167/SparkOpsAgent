package command_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/stretchr/testify/require"
)

func TestDefaultMenuRolePolicies_includeAdminOnRestrictedPaths(t *testing.T) {
	restricted := []string{
		"/config/secrets",
		"/delivery/pipelines",
		"/config/webhooks",
		"/plugins/kubernetes/access-controls",
	}
	for _, path := range restricted {
		roles, ok := command.DefaultMenuRolePolicies[path]
		require.Truef(t, ok, "missing policy for %s", path)
		require.Containsf(t, roles, "admin", "%s must include admin", path)
		require.Containsf(t, roles, "operator", "%s must include operator", path)
	}
}

func TestDefaultMenuRolePolicies_templatesHiddenFromViewer(t *testing.T) {
	roles := command.DefaultMenuRolePolicies["/templates"]
	require.NotContains(t, roles, "viewer")
	require.Contains(t, roles, "developer")
}
