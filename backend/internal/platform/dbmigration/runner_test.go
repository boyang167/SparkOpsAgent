package dbmigration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSourceCombinesDuplicateVersionsSymmetrically(t *testing.T) {
	source := t.TempDir()
	writeMigrationTestFile(t, source, "000061_alpha.up.sql", "UP ALPHA;")
	writeMigrationTestFile(t, source, "000061_alpha.down.sql", "DOWN ALPHA;")
	writeMigrationTestFile(t, source, "000061_beta.up.sql", "UP BETA;")
	writeMigrationTestFile(t, source, "000061_beta.down.sql", "DOWN BETA;")

	normalized, cleanup, err := normalizeSource(source, 0)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	up, err := os.ReadFile(filepath.Join(normalized, "000061_combined.up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(normalized, "000061_combined.down.sql"))
	require.NoError(t, err)

	require.Less(t, strings.Index(string(up), "UP ALPHA"), strings.Index(string(up), "UP BETA"))
	require.Less(t, strings.Index(string(down), "DOWN BETA"), strings.Index(string(down), "DOWN ALPHA"))
}

func writeMigrationTestFile(t *testing.T, directory, name, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600))
}
