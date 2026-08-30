package schema_test

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openSQLiteK8sSchema(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin_kubernetes").Error)
	require.NoError(t, k8sschema.AutoMigrate(db))
	return db
}

func TestAutoMigrateCreatesPluginKubernetesTables(t *testing.T) {
	db := openSQLiteK8sSchema(t)

	for _, table := range []string{"cluster_settings", "records", "diagnosis"} {
		var count int64
		require.NoError(t, db.Raw(
			"SELECT count(*) FROM plugin_kubernetes.sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count).Error)
		require.Equalf(t, int64(1), count, "missing table plugin_kubernetes.%s", table)
	}
}

func TestClusterSettingsHasNoKubeconfigFields(t *testing.T) {
	db := openSQLiteK8sSchema(t)

	var columns []struct {
		Name string
	}
	require.NoError(t, db.Raw("PRAGMA plugin_kubernetes.table_info('cluster_settings')").Scan(&columns).Error)
	require.NotEmpty(t, columns)

	names := make([]string, 0, len(columns))
	for _, col := range columns {
		names = append(names, strings.ToLower(col.Name))
	}

	forbidden := []string{"kubeconfig", "ca", "cert", "key", "token"}
	for _, forbiddenName := range forbidden {
		for _, name := range names {
			require.NotEqualf(t, forbiddenName, name, "cluster_settings must not store %s", forbiddenName)
		}
	}

	require.Contains(t, names, "cluster_id")
	require.Contains(t, names, "prometheus_url")
	require.Contains(t, names, "velaux_url")
	require.Contains(t, names, "llm_base_url")
}
