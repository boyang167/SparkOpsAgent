package migrations

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationIntegrationRequiredRecognizesCommonTruthyValues(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", " True "} {
		t.Run(value, func(t *testing.T) {
			require.True(t, migrationIntegrationRequired(value))
		})
	}
	for _, value := range []string{"", "0", "false", "yes"} {
		t.Run("false_"+value, func(t *testing.T) {
			require.False(t, migrationIntegrationRequired(value))
		})
	}
}

func TestMigrationIntegrationGateSubprocess(t *testing.T) {
	if os.Getenv("SPARKOPS_MIGRATION_GATE_CHILD") == "1" {
		openMigrationPostgres(t)
		return
	}

	run := func(requireIntegration string) ([]byte, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestMigrationIntegrationGateSubprocess$")
		env := make([]string, 0, len(os.Environ())+2)
		for _, item := range os.Environ() {
			if strings.HasPrefix(item, "DATABASE_DSN=") ||
				strings.HasPrefix(item, "REQUIRE_INTEGRATION=") ||
				strings.HasPrefix(item, "SPARKOPS_MIGRATION_GATE_CHILD=") {
				continue
			}
			env = append(env, item)
		}
		env = append(env, "SPARKOPS_MIGRATION_GATE_CHILD=1")
		if requireIntegration != "" {
			env = append(env, "REQUIRE_INTEGRATION="+requireIntegration)
		}
		cmd.Env = env
		return cmd.CombinedOutput()
	}

	output, err := run("")
	require.NoError(t, err, string(output))

	for _, value := range []string{"1", "true", "TRUE"} {
		output, err = run(value)
		require.Error(t, err, value)
		require.Contains(t, string(output), "DATABASE_DSN is required", value)
	}
}
