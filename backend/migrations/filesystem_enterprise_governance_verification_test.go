package migrations

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFilesystemEnterpriseGovernanceMigratesExistingDataFrom82To87AndBack(t *testing.T) {
	db := openMigrationPostgres(t)
	for _, migration := range []string{
		"000077_filesystem_kernel.up.sql",
		"000078_filesystem_transfer.up.sql",
		"000079_filesystem_storage_fabric.up.sql",
		"000080_knowledge_filesystem_expand.up.sql",
		"000081_filesystem_transfer_list.up.sql",
		"000082_filesystem_blob_lifecycle.up.sql",
	} {
		require.NoErrorf(t, db.Exec(readMigration(t, migration)).Error, "apply %s", migration)
	}

	organizationID, workspaceID := uuid.New(), uuid.New()
	namespaceID, rootID, fileID := uuid.New(), uuid.New(), uuid.New()
	ownerID, recipientID, legacyGrantID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.namespaces
			(id, organization_id, workspace_id, root_node_id, status)
		VALUES (?, ?, ?, NULL, 'ACTIVE')
	`, namespaceID, organizationID, workspaceID).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.nodes
			(id, namespace_id, parent_id, node_type, name, normalized_name, state, owner_id)
		VALUES
			(?, ?, NULL, 'DIRECTORY', '.root', '.root', 'ACTIVE', ?),
			(?, ?, ?, 'FILE', 'legacy.txt', 'legacy.txt', 'ACTIVE', ?)
	`, rootID, namespaceID, ownerID, fileID, namespaceID, rootID, ownerID).Error)
	require.NoError(t, db.Exec(`
		UPDATE filesystem.namespaces SET root_node_id = ? WHERE id = ?
	`, rootID, namespaceID).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.quota_accounts
			(id, organization_id, workspace_id, used_bytes, reserved_bytes, used_files, reserved_files, version)
		VALUES (?, ?, ?, 0, 0, 0, 0, 0)
	`, uuid.New(), organizationID, workspaceID).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.resource_grants
			(id, node_id, grantee_id, permission, effect)
		VALUES (?, ?, ?, 'READ', 'ALLOW')
	`, legacyGrantID, fileID, recipientID).Error)

	for _, migration := range []string{
		"000083_filesystem_namespace_types.up.sql",
		"000084_filesystem_grant_principals.up.sql",
		"000085_filesystem_share_requests.up.sql",
		"000086_filesystem_namespace_quota.up.sql",
		"000087_filesystem_file_lineage.up.sql",
	} {
		require.NoErrorf(t, db.Exec(readMigration(t, migration)).Error, "apply %s", migration)
	}

	var workspaceNamespaces, userGrants, legacyFiles, legacyActiveGrants int64
	require.NoError(t, db.Table("filesystem.namespaces").Where("namespace_type = 'WORKSPACE'").Count(&workspaceNamespaces).Error)
	require.NoError(t, db.Table("filesystem.resource_grants").Where("principal_type = 'USER'").Count(&userGrants).Error)
	require.NoError(t, db.Table("filesystem.nodes").Where("id = ? AND name = 'legacy.txt'", fileID).Count(&legacyFiles).Error)
	require.NoError(t, db.Table("filesystem.resource_grants").Where("id = ? AND revoked_at IS NULL", legacyGrantID).Count(&legacyActiveGrants).Error)
	require.Equal(t, int64(1), workspaceNamespaces)
	require.Equal(t, int64(1), userGrants)
	require.Equal(t, int64(1), legacyFiles)
	require.Equal(t, int64(1), legacyActiveGrants)

	typedGrantID := uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.resource_grants
			(id, node_id, organization_id, grantee_id, principal_type, principal_id,
			 permission, effect, inherit_to_children, policy_version)
		VALUES (?, ?, ?, ?, 'WORKSPACE', ?, 'READ', 'ALLOW', TRUE, 1)
	`, typedGrantID, fileID, organizationID, workspaceID, workspaceID).Error)
	require.NoError(t, db.Exec(`
		UPDATE filesystem.resource_grants SET revoked_at = NOW()
		WHERE id = ? AND revoked_at IS NULL
	`, typedGrantID).Error)
	var typedGrantActive int64
	require.NoError(t, db.Table("filesystem.resource_grants").Where("id = ? AND revoked_at IS NULL", typedGrantID).Count(&typedGrantActive).Error)
	require.Zero(t, typedGrantActive)

	downloadGrantID := uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.resource_grants
			(id, node_id, organization_id, grantee_id, principal_type, principal_id,
			 permission, effect, inherit_to_children, policy_version)
		VALUES (?, ?, ?, ?, 'USER', ?, 'DOWNLOAD', 'ALLOW', FALSE, 1)
	`, downloadGrantID, fileID, organizationID, recipientID, recipientID).Error)

	for _, migration := range []string{
		"000087_filesystem_file_lineage.down.sql",
		"000086_filesystem_namespace_quota.down.sql",
		"000085_filesystem_share_requests.down.sql",
		"000084_filesystem_grant_principals.down.sql",
		"000083_filesystem_namespace_types.down.sql",
	} {
		require.NoErrorf(t, db.Exec(readMigration(t, migration)).Error, "apply %s", migration)
	}

	var filesAfterDown, legacyGrantsAfterDown, downloadGrantsAfterDown int64
	require.NoError(t, db.Table("filesystem.nodes").Where("id = ? AND name = 'legacy.txt'", fileID).Count(&filesAfterDown).Error)
	require.NoError(t, db.Table("filesystem.resource_grants").Where("id = ?", legacyGrantID).Count(&legacyGrantsAfterDown).Error)
	require.NoError(t, db.Table("filesystem.resource_grants").Where("id = ?", downloadGrantID).Count(&downloadGrantsAfterDown).Error)
	require.Equal(t, int64(1), filesAfterDown)
	require.Equal(t, int64(1), legacyGrantsAfterDown)
	require.Zero(t, downloadGrantsAfterDown)

	t.Logf(
		"82->87 rows: workspace_namespaces=%d user_grants=%d legacy_files=%d legacy_active_grants=%d typed_active_after_revoke=%d; 87->82 rows: legacy_files=%d legacy_grants=%d download_grants=%d",
		workspaceNamespaces, userGrants, legacyFiles, legacyActiveGrants, typedGrantActive,
		filesAfterDown, legacyGrantsAfterDown, downloadGrantsAfterDown,
	)
}
