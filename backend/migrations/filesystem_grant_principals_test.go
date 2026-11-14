package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemGrantPrincipalMigrationDefinesBackfillAndActiveUniqueness(t *testing.T) {
	up := readMigration(t, "000084_filesystem_grant_principals.up.sql")
	require.Contains(t, up, "principal_type")
	require.Contains(t, up, "principal_id")
	require.Contains(t, up, "organization_id")
	require.Contains(t, up, "inherit_to_children")
	require.Contains(t, up, "policy_version")
	require.Contains(t, up, "revoked_at")
	require.Contains(t, up, "principal_type = 'USER'")
	require.Contains(t, up, "inherit_to_children = TRUE")
	require.Contains(t, up, "policy_version = 1")
	require.Contains(t, up, "FROM filesystem.nodes")
	require.Contains(t, up, "JOIN filesystem.namespaces")
	require.Contains(t, up, "WHERE revoked_at IS NULL")
	require.Contains(t, up, "node_id, principal_type, principal_id, permission, effect")
}
