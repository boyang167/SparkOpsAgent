package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectDeployPipelineMigrationEnforcesWorkspaceClusterAuthorization(t *testing.T) {
	up := readMigration(t, "000032_project_deploy_pipeline.up.sql")

	require.Contains(t, up, "PRIMARY KEY (workspace_id, cluster_id)")
	requireInOrder(t, up,
		"CREATE TABLE IF NOT EXISTS platform_admin.workspace_clusters",
		"cluster_id   UUID NOT NULL",
		"REFERENCES platform_admin.cluster_info(id) ON DELETE CASCADE",
	)
	requireInOrder(t, up,
		"CREATE TABLE IF NOT EXISTS platform_admin.workspace_clusters",
		"PRIMARY KEY (workspace_id, cluster_id)",
		"ALTER TABLE workspace_app.projects",
		"ADD CONSTRAINT fk_projects_default_cluster_workspace",
		"FOREIGN KEY (workspace_id, default_cluster_id)",
		"REFERENCES platform_admin.workspace_clusters(workspace_id, cluster_id)",
	)
	requireInOrder(t, up,
		"CREATE TABLE IF NOT EXISTS delivery.deployment_runs",
		"ALTER TABLE delivery.deployment_runs",
		"ADD CONSTRAINT fk_deployment_runs_workspace_cluster",
		"FOREIGN KEY (workspace_id, cluster_id)",
		"REFERENCES platform_admin.workspace_clusters(workspace_id, cluster_id)",
	)
}

func TestProjectDeployPipelineMigrationIdempotentlyAddsNamedCascadeFKs(t *testing.T) {
	up := readMigration(t, "000032_project_deploy_pipeline.up.sql")

	require.Equal(t, 2, strings.Count(up, "DO $$"))
	requireInOrder(t, up,
		"CREATE TABLE IF NOT EXISTS platform_admin.workspace_clusters",
		"CONSTRAINT fk_workspace_clusters_cluster_info",
		"FOREIGN KEY (cluster_id)",
		"REFERENCES platform_admin.cluster_info(id) ON DELETE CASCADE",
		"DO $$",
		"FROM pg_constraint c",
		"JOIN pg_class rel ON rel.oid = c.conrelid",
		"JOIN pg_namespace ns ON ns.oid = rel.relnamespace",
		"c.conname = 'fk_workspace_clusters_cluster_info'",
		"c.conrelid = 'platform_admin.workspace_clusters'::regclass",
		"ns.nspname = 'platform_admin'",
		"rel.relname = 'workspace_clusters'",
		"ALTER TABLE platform_admin.workspace_clusters",
		"ADD CONSTRAINT fk_workspace_clusters_cluster_info",
		"REFERENCES platform_admin.cluster_info(id) ON DELETE CASCADE",
	)
	requireInOrder(t, up,
		"CREATE TABLE IF NOT EXISTS delivery.deployment_run_events",
		"CONSTRAINT fk_deployment_run_events_run",
		"FOREIGN KEY (run_id)",
		"REFERENCES delivery.deployment_runs(id) ON DELETE CASCADE",
		"DO $$",
		"FROM pg_constraint c",
		"JOIN pg_class rel ON rel.oid = c.conrelid",
		"JOIN pg_namespace ns ON ns.oid = rel.relnamespace",
		"c.conname = 'fk_deployment_run_events_run'",
		"c.conrelid = 'delivery.deployment_run_events'::regclass",
		"ns.nspname = 'delivery'",
		"rel.relname = 'deployment_run_events'",
		"ALTER TABLE delivery.deployment_run_events",
		"ADD CONSTRAINT fk_deployment_run_events_run",
		"REFERENCES delivery.deployment_runs(id) ON DELETE CASCADE",
	)
}

func TestProjectDeployPipelineDownMigrationDropsCompositeFKsBeforeDependencies(t *testing.T) {
	down := readMigration(t, "000032_project_deploy_pipeline.down.sql")

	requireInOrder(t, down,
		"ALTER TABLE workspace_app.projects",
		"DROP CONSTRAINT IF EXISTS fk_projects_default_cluster_workspace",
		"ALTER TABLE delivery.deployment_runs",
		"DROP CONSTRAINT IF EXISTS fk_deployment_runs_workspace_cluster",
		"DROP TABLE IF EXISTS platform_admin.workspace_clusters",
		"DROP COLUMN IF EXISTS default_cluster_id",
	)
	requireInOrder(t, down,
		"DROP CONSTRAINT IF EXISTS fk_projects_default_cluster_workspace",
		"DROP CONSTRAINT IF EXISTS fk_deployment_run_events_run",
		"DROP TABLE IF EXISTS delivery.deployment_run_events",
		"DROP CONSTRAINT IF EXISTS fk_deployment_runs_workspace_cluster",
		"DROP TABLE IF EXISTS delivery.deployment_runs",
		"DROP CONSTRAINT IF EXISTS fk_workspace_clusters_cluster_info",
		"DROP TABLE IF EXISTS platform_admin.workspace_clusters",
	)
}

func TestProjectDeployPipelineMigrationAddsOptimisticLockVersion(t *testing.T) {
	up := readMigration(t, "000032_project_deploy_pipeline.up.sql")
	require.Contains(t, up, "version")
	require.Contains(t, up, "BIGINT NOT NULL DEFAULT 0")
	require.Contains(t, up, "attempt_count")
	require.Contains(t, up, "INT NOT NULL DEFAULT 0")
	require.Contains(t, up, "lease_owner")
	require.Contains(t, up, "lease_expires_at")
	require.Contains(t, up, "execution_started_at")
	down := readMigration(t, "000032_project_deploy_pipeline.down.sql")
	require.Contains(t, down, "DROP COLUMN IF EXISTS execution_started_at")
}

func TestProjectDeployPipelineMigrationPersistsStreamEventStatus(t *testing.T) {
	up := readMigration(t, "000032_project_deploy_pipeline.up.sql")

	requireInOrder(t, up,
		"CREATE TABLE IF NOT EXISTS delivery.deployment_run_events",
		"status  VARCHAR(16) NOT NULL",
		"stage   VARCHAR(16) NOT NULL",
	)
}

func TestTransactionalEventsMigrationDefinesReliabilityConstraints(t *testing.T) {
	up := readMigration(t, "000070_transactional_events.up.sql")
	require.Contains(t, up, "CREATE TABLE IF NOT EXISTS platform.event_outbox")
	require.Contains(t, up, "CREATE TABLE IF NOT EXISTS platform.event_inbox")
	require.Contains(t, up, "CREATE TABLE IF NOT EXISTS platform.operation_outbox")
	require.Contains(t, up, "PRIMARY KEY (event_id, subscriber)")
	require.Contains(t, up, "CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_outbox_idempotency_key")
	require.Contains(t, up, "CREATE INDEX IF NOT EXISTS ix_event_outbox_claim")
}

func TestProductWorkflowRunVersionMigrationIsExistingRowSafe(t *testing.T) {
	up := readMigration(t, "000071_product_workflow_run_version.up.sql")
	requireInOrder(t, up,
		"ADD COLUMN IF NOT EXISTS version BIGINT",
		"UPDATE product.product_workflow_runs",
		"SET version = 0",
		"WHERE version IS NULL",
		"ALTER COLUMN version SET DEFAULT 0",
		"ALTER COLUMN version SET NOT NULL",
	)
	down := readMigration(t, "000071_product_workflow_run_version.down.sql")
	require.Contains(t, down, "DROP COLUMN IF EXISTS version")
}

func TestProjectArchiveGenerationMigrationIsExistingRowSafe(t *testing.T) {
	up := readMigration(t, "000072_project_archive_generation.up.sql")
	require.Contains(t, up, "ADD COLUMN IF NOT EXISTS archive_generation UUID")
	require.NotContains(t, up, "NOT NULL")
	require.Contains(t, up, "WHERE status = 'archiving'")
	require.Contains(t, up, "AND archive_generation IS NULL")
	require.Contains(t, up, "md5('workspace-archive:' || id::text)")
	require.Contains(t, up, ")::uuid")
	down := readMigration(t, "000072_project_archive_generation.down.sql")
	require.Contains(t, down, "DROP COLUMN IF EXISTS archive_generation")
}

func TestDeploymentRunArchiveGenerationMigrationIsExistingRowSafe(t *testing.T) {
	up := readMigration(t, "000073_deployment_run_archive_generation.up.sql")
	require.Contains(t, up, "ALTER TABLE delivery.deployment_runs")
	require.Contains(t, up, "ADD COLUMN IF NOT EXISTS archive_generation UUID")
	require.NotContains(t, up, "NOT NULL")
	require.Contains(t, up, "UPDATE workspace_app.projects")
	require.Contains(t, up, "WHERE status = 'archiving'")
	require.Contains(t, up, "AND archive_generation IS NULL")

	down := readMigration(t, "000073_deployment_run_archive_generation.down.sql")
	require.Contains(t, down, "DROP COLUMN IF EXISTS archive_generation")
}

func TestProjectArchivePhaseMigrationDefinesFenceStateMachine(t *testing.T) {
	up := readMigration(t, "000074_project_archive_phase.up.sql")
	require.Contains(t, up, "ADD COLUMN IF NOT EXISTS archive_phase")
	require.Contains(t, up, "DEFAULT 'none'")
	require.Contains(t, up, "UPDATE workspace_app.projects")
	require.Contains(t, up, "status = 'archiving'")
	for _, phase := range []string{
		"none", "pending", "undeploy_claimed", "undeploying",
		"pipeline_pending", "pipeline_claimed", "finalizing",
	} {
		require.Contains(t, up, "'"+phase+"'")
	}
	require.Contains(t, up, "CHECK (archive_phase IN")

	down := readMigration(t, "000074_project_archive_phase.down.sql")
	require.Contains(t, down, "DROP COLUMN IF EXISTS archive_phase")
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(contents)
}

func requireInOrder(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	offset := 0
	for _, needle := range needles {
		index := strings.Index(haystack[offset:], needle)
		require.NotEqualf(t, -1, index, "expected %q after offset %d", needle, offset)
		offset += index + len(needle)
	}
}
