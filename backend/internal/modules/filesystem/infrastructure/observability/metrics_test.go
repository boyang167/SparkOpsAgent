package observability_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/observability"
)

func TestFilesystemMetricNamesAreStable(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	require.NoError(t, observability.Register(registry))

	names := gatherNames(t, registry)
	for _, name := range []string{
		"sparkops_filesystem_transfer_active",
		"sparkops_filesystem_transfer_bytes_total",
		"sparkops_filesystem_transfer_failures_total",
		"sparkops_filesystem_transfer_state_duration_seconds",
		"sparkops_filesystem_scanner_latency_seconds",
		"sparkops_filesystem_scanner_rejections_total",
		"sparkops_filesystem_queue_lag_seconds",
		"sparkops_filesystem_provider_errors_total",
		"sparkops_filesystem_provider_latency_seconds",
		"sparkops_filesystem_quota_reserved_bytes",
		"sparkops_filesystem_quota_used_bytes",
		"sparkops_filesystem_replica_inconsistency",
		"sparkops_filesystem_gc_backlog",
		"sparkops_filesystem_reconcile_backlog",
	} {
		assert.Contains(t, names, name)
	}
}

func TestFilesystemMetricsCanRegisterTwice(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	require.NoError(t, observability.Register(registry))
	require.NoError(t, observability.Register(registry))
}

func gatherNames(t *testing.T, registry *prometheus.Registry) map[string]struct{} {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)
	names := make(map[string]struct{}, len(families))
	for _, family := range families {
		names[family.GetName()] = struct{}{}
	}
	return names
}
