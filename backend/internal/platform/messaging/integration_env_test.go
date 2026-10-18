//go:build integration

package messaging

import (
	"os"
	"strings"
	"testing"
)

func requireIntegrationEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	if strings.EqualFold(os.Getenv("CI"), "true") || strings.EqualFold(os.Getenv("REQUIRE_INTEGRATION"), "true") {
		t.Fatalf("%s is required when integration tests run in CI", name)
	}
	t.Skipf("%s is required for integration tests", name)
	return ""
}
