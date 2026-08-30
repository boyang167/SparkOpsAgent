package bootstrap

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunRegistersPluginModule(t *testing.T) {
	source, err := os.ReadFile("app.go")
	require.NoError(t, err)
	app := string(source)

	require.Contains(t, app, `pluginmod "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin"`)
	require.Contains(t, app, "pluginmod.New(tokens)")
	require.Less(t, strings.Index(app, "pluginmod.New(tokens)"), strings.Index(app, "sparkplugin.InstallAll(c.Engine, logger)"))
}
