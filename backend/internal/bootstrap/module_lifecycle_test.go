package bootstrap

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunDefersModuleCleanupBeforeRegistration(t *testing.T) {
	source, err := os.ReadFile("app.go")
	require.NoError(t, err)
	cleanupAt := indexOf(source, "defer closeModules(modules, logger)")
	registerAt := indexOf(source, "for _, m := range modules")
	require.NotEqual(t, -1, cleanupAt)
	require.NotEqual(t, -1, registerAt)
	require.Less(t, cleanupAt, registerAt)
}

func indexOf(source []byte, value string) int {
	for i := 0; i+len(value) <= len(source); i++ {
		if string(source[i:i+len(value)]) == value {
			return i
		}
	}
	return -1
}
