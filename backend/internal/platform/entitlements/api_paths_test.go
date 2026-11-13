package entitlements

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemAPIPathEntitlement(t *testing.T) {
	for _, path := range []string{
		"/api/v1/workspaces/ws-1/files",
		"/api/v1/workspaces/ws-1/files?sort=name",
		"/api/v1/workspaces/ws-1/files/",
		"/api/v1/workspaces/ws-1/files////?sort=name",
		"/api/v1/workspaces/ws-1/namespaces",
		"/api/v1/workspaces/ws-1/namespaces/",
		"/api/v1/workspaces/ws-1/directories",
		"/api/v1/workspaces/ws-1/directories/",
		"/api/v1/workspaces/ws-1/transfers",
		"/api/v1/workspaces/ws-1/transfers/",
		"/api/v1/workspaces/ws-1/%66iles",
		"/api/v1/workspaces%2Fws-1/files",
	} {
		t.Run(path, func(t *testing.T) {
			require.Equal(t, "filesystem", APIPathEntitlement(path))
		})
	}
}

func TestFilesystemAPIPathEntitlementRejectsSimilarAndNonWorkspacePaths(t *testing.T) {
	for _, path := range []string{
		"/api/v1/workspaces/ws-1/files-legacy",
		"/api/v1/workspaces/ws-1/namespaces-old",
		"/api/v1/workspaces/ws-1/transfers-v2",
		"/api/v1/workspaces/ws-1/%66iles-legacy",
		"/api%2Fv1/workspaces/ws-1/files",
		"/api/v1/workspaces/ws-1/%ZZ",
		"/api/v1/workspaces/ws-1/files?sort=%ZZ",
		"/api/v1/files",
		"/api/v1/namespaces",
		"/api/v1/transfers",
		"/api/v1/orgs/org-1/workspaces/ws-1/files",
	} {
		t.Run(path, func(t *testing.T) {
			require.Empty(t, APIPathEntitlement(path))
		})
	}
}

func TestFilesystemAllowsModule(t *testing.T) {
	require.True(t, (WorkspaceEntitlements{Filesystem: true}).AllowsModule("filesystem"))
	require.False(t, (WorkspaceEntitlements{Filesystem: false}).AllowsModule("filesystem"))
}

func TestFilesystemEntitlementDefaultsAndJSONCompatibility(t *testing.T) {
	defaults := Default()
	require.True(t, defaults.Filesystem)

	var missing WorkspaceEntitlements
	require.NoError(t, json.Unmarshal([]byte(`{"agent":true}`), &missing))
	require.True(t, missing.Filesystem)

	var disabled WorkspaceEntitlements
	require.NoError(t, json.Unmarshal([]byte(`{"filesystem":false}`), &disabled))
	require.False(t, disabled.Filesystem)

	roundTrip, err := json.Marshal(disabled)
	require.NoError(t, err)
	var decoded WorkspaceEntitlements
	require.NoError(t, json.Unmarshal(roundTrip, &decoded))
	require.False(t, decoded.Filesystem)
}
