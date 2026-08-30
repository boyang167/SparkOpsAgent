package persistence

import (
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

	bindingSchema := parseGormSchema(t, &workspaceBindingModel{})
	for _, fieldName := range []string{"WorkspaceID", "PluginID", "Enabled"} {
		field := bindingSchema.LookUpField(fieldName)
		require.NotNil(t, field, "missing binding field %s", fieldName)
		require.Contains(t, field.TagSettings, "NOT NULL", "binding.%s not null", field.DBName)
	}
	enabled := bindingSchema.LookUpField("Enabled")
	require.Equal(t, "false", strings.ToLower(enabled.TagSettings["DEFAULT"]))
}

func parseGormSchema(t *testing.T, value any) *schema.Schema {
	t.Helper()
	parsed, err := schema.Parse(value, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	return parsed
}
