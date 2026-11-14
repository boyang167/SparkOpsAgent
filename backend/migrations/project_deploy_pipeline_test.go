package migrations

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/bootstrap"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/dbmigration"
	messagepersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func TestProjectArchiveIntentMigrationBackfillsConservatively(t *testing.T) {
	up := readMigration(t, "000075_project_archive_intent.up.sql")
	require.Contains(t, up, "ADD COLUMN IF NOT EXISTS archive_allow_undeploy BOOLEAN")
	require.Contains(t, up, "WHERE status = 'archiving'")
	require.Contains(t, up, "archive_allow_undeploy IS NULL")
	requireInOrder(t, up,
		"SET archive_allow_undeploy = CASE",
		"WHEN archive_phase = 'pending' THEN FALSE",
		"ELSE TRUE",
		"END",
	)
	require.Contains(t, up, "CHECK")
	require.Contains(t, up, "conrelid = 'workspace_app.projects'::regclass")

	down := readMigration(t, "000075_project_archive_intent.down.sql")
	require.Contains(t, down, "DROP COLUMN IF EXISTS archive_allow_undeploy")
}

func TestProjectArchiveIntentMigrationUpgradesPhaseSemanticsPostgres(t *testing.T) {
	db := openMigrationPostgres(t)
	require.NoError(t, db.Exec("CREATE SCHEMA workspace_app").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE workspace_app.projects (
			id UUID PRIMARY KEY,
			status VARCHAR(32) NOT NULL,
			archive_phase VARCHAR(32) NOT NULL DEFAULT 'none'
		)
	`).Error)

	expected := map[string]bool{
		"pending":          false,
		"undeploy_claimed": true,
		"undeploying":      true,
		"pipeline_pending": true,
		"pipeline_claimed": true,
		"finalizing":       true,
	}
	for phase := range expected {
		require.NoError(t, db.Exec(
			"INSERT INTO workspace_app.projects (id, status, archive_phase) VALUES (?, 'archiving', ?)",
			uuid.NewString(), phase,
		).Error)
	}
	activeID, archivedID := uuid.NewString(), uuid.NewString()
	require.NoError(t, db.Exec(
		"INSERT INTO workspace_app.projects (id, status, archive_phase) VALUES (?, 'active', 'none'), (?, 'archived', 'none')",
		activeID, archivedID,
	).Error)

	require.NoError(t, db.Exec(readMigration(t, "000075_project_archive_intent.up.sql")).Error)

	var rows []struct {
		ArchivePhase         string
		ArchiveAllowUndeploy *bool
	}
	require.NoError(t, db.Table("workspace_app.projects").
		Where("status = 'archiving'").
		Find(&rows).Error)
	require.Len(t, rows, len(expected))
	for _, row := range rows {
		require.NotNil(t, row.ArchiveAllowUndeploy, row.ArchivePhase)
		require.Equal(t, expected[row.ArchivePhase], *row.ArchiveAllowUndeploy, row.ArchivePhase)
	}

	var terminalNullCount int64
	require.NoError(t, db.Table("workspace_app.projects").
		Where("id IN ?", []string{activeID, archivedID}).
		Where("archive_allow_undeploy IS NULL").
		Count(&terminalNullCount).Error)
	require.Equal(t, int64(2), terminalNullCount)

	var columnDefault *string
	require.NoError(t, db.Raw(`
		SELECT column_default
		FROM information_schema.columns
		WHERE table_schema = 'workspace_app'
		  AND table_name = 'projects'
		  AND column_name = 'archive_allow_undeploy'
	`).Scan(&columnDefault).Error)
	require.Nil(t, columnDefault)
}

func TestSQLMigrationRunnerFreshReaches75AndPassesStartupGate(t *testing.T) {
	db := openMigrationPostgres(t)
	require.NoError(t, bootstrap.AutoMigrate(db))
	require.NoError(t, dbmigration.RunFrom(currentMigrationDSN(t, db), ".", 70, 0))
	require.NoError(t, messagepersistence.ValidateTransactionalEventSchema(db))
}

func TestSQLMigrationRunnerUpgrades69And74To75WithStartupGate(t *testing.T) {
	for _, startingVersion := range []uint{69, 74} {
		t.Run(fmt.Sprintf("from-%d", startingVersion), func(t *testing.T) {
			db := openMigrationPostgres(t)
			dsn := currentMigrationDSN(t, db)
			require.NoError(t, bootstrap.AutoMigrate(db))
			require.NoError(t, dbmigration.RunFrom(dsn, ".", 70, 69))
			if startingVersion > 69 {
				require.NoError(t, dbmigration.RunFrom(dsn, ".", 70, startingVersion))
			}
			require.ErrorContains(t, messagepersistence.ValidateTransactionalEventSchema(db), "required version 75")
			require.NoError(t, dbmigration.RunFrom(dsn, ".", 70, 0))
			require.NoError(t, messagepersistence.ValidateTransactionalEventSchema(db))
		})
	}
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(contents)
}

func openMigrationPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		if migrationIntegrationRequired(os.Getenv("CI")) ||
			migrationIntegrationRequired(os.Getenv("REQUIRE_INTEGRATION")) {
			t.Fatal("DATABASE_DSN is required when migration integration tests are required")
		}
		t.Skip("DATABASE_DSN is not set")
	}
	admin, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	databaseName := "sparkops_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE DATABASE " + databaseName).Error; err != nil {
		if migrationIntegrationRequired(os.Getenv("CI")) ||
			migrationIntegrationRequired(os.Getenv("REQUIRE_INTEGRATION")) {
			t.Fatalf("cannot create isolated PostgreSQL test database: %v", err)
		}
		t.Skipf("cannot create isolated PostgreSQL test database: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(migrationDSNWithDatabase(t, dsn, databaseName)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, admin.Exec("DROP DATABASE "+databaseName).Error)
		adminSQL, adminErr := admin.DB()
		require.NoError(t, adminErr)
		require.NoError(t, adminSQL.Close())
	})
	return db
}

func migrationIntegrationRequired(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true
	default:
		return false
	}
}

func currentMigrationDSN(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var databaseName string
	require.NoError(t, db.Raw("SELECT current_database()").Scan(&databaseName).Error)
	return migrationDSNWithDatabase(t, os.Getenv("DATABASE_DSN"), databaseName)
}

func migrationDSNWithDatabase(t *testing.T, dsn, databaseName string) string {
	t.Helper()
	if strings.Contains(dsn, "://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		parsed.Path = "/" + databaseName
		return parsed.String()
	}
	databaseSetting := regexp.MustCompile(`(?i)\b(?:dbname|database)\s*=\s*(?:'[^']*'|\S+)`)
	if databaseSetting.MatchString(dsn) {
		return databaseSetting.ReplaceAllString(dsn, "dbname="+databaseName)
	}
	return dsn + " dbname=" + databaseName
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
