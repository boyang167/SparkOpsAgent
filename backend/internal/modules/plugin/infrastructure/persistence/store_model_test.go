package persistence

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func TestGormModelsDeclareRequiredSchemaConstraints(t *testing.T) {
	pluginSchema := parseGormSchema(t, &pluginModel{})
	for _, fieldName := range []string{"Version", "DisplayName", "ManifestYAML", "Status"} {
		field := pluginSchema.LookUpField(fieldName)
		require.NotNil(t, field, "missing plugin field %s", fieldName)
		require.Equal(t, "text", strings.ToLower(field.TagSettings["TYPE"]), "plugin.%s type", field.DBName)
		require.Contains(t, field.TagSettings, "NOT NULL", "plugin.%s not null", field.DBName)
	}
	createdBy := pluginSchema.LookUpField("CreatedBy")
	require.NotNil(t, createdBy)
	require.Equal(t, "uuid", strings.ToLower(createdBy.TagSettings["TYPE"]))
	require.Contains(t, createdBy.TagSettings, "NOT NULL")
	installedFiles := pluginSchema.LookUpField("InstalledFilesJSON")
	require.NotNil(t, installedFiles)
	require.Equal(t, "jsonb", strings.ToLower(installedFiles.TagSettings["TYPE"]))
	require.Contains(t, installedFiles.TagSettings, "NOT NULL")
	packageDigest := pluginSchema.LookUpField("PackageDigest")
	require.NotNil(t, packageDigest)
	require.Equal(t, "text", strings.ToLower(packageDigest.TagSettings["TYPE"]))
	require.Contains(t, packageDigest.TagSettings, "NOT NULL")

	bindingSchema := parseGormSchema(t, &workspaceBindingModel{})
	for _, fieldName := range []string{"WorkspaceID", "PluginID", "Enabled"} {
		field := bindingSchema.LookUpField(fieldName)
		require.NotNil(t, field, "missing binding field %s", fieldName)
		require.Contains(t, field.TagSettings, "NOT NULL", "binding.%s not null", field.DBName)
	}
	enabled := bindingSchema.LookUpField("Enabled")
	require.Equal(t, "false", strings.ToLower(enabled.TagSettings["DEFAULT"]))

	receiptSchema := parseGormSchema(t, &uninstallReceiptModel{})
	pluginID := receiptSchema.LookUpField("PluginID")
	require.NotNil(t, pluginID)
	require.True(t, pluginID.PrimaryKey)
	require.NotNil(t, receiptSchema.LookUpField("DeployedAt"))
}

func TestPluginMigrationDeclaresInstalledFilesJSONB(t *testing.T) {
	migrations := filepath.Join("..", "..", "..", "..", "..", "migrations")
	source, err := os.ReadFile(filepath.Join(migrations, "000033_plugin_framework.up.sql"))
	require.NoError(t, err)
	require.Contains(t, string(source), "installed_files JSONB NOT NULL DEFAULT '[]'::jsonb")
	require.Contains(t, string(source), "package_digest TEXT NOT NULL")
	require.Contains(t, string(source), "CREATE TABLE plugin.uninstall_receipts")

	source, err = os.ReadFile(filepath.Join(migrations, "000033_plugin_framework.down.sql"))
	require.NoError(t, err)
	require.Contains(t, string(source), "DROP TABLE IF EXISTS plugin.uninstall_receipts")
	require.Contains(t, string(source), "DROP COLUMN IF EXISTS installed_files")
}

func parseGormSchema(t *testing.T, value any) *schema.Schema {
	t.Helper()
	parsed, err := schema.Parse(value, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	return parsed
}
