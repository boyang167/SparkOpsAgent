package messaging

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestMetricsRegistersRequiredCollectorsOnce(t *testing.T) {
	registry := prometheus.NewRegistry()

	first, err := NewMetrics(registry)
	require.NoError(t, err)
	second, err := NewMetrics(registry)
	require.NoError(t, err)
	require.Same(t, first, second)

	first.OutboxPending.Set(2)
	first.EventPublish.WithLabelValues("success").Inc()
	first.EventDispatcherError.WithLabelValues("stats").Inc()
	first.OperationDispatcherError.WithLabelValues("claim").Inc()
	first.EventConsume.WithLabelValues("test.subscriber", "success").Inc()
	first.EventRetry.WithLabelValues("test.subscriber", "5s").Inc()
	first.EventDLQ.WithLabelValues("test.subscriber").Inc()
	first.InboxDuplicate.WithLabelValues("test.subscriber").Inc()

	families, err := registry.Gather()
	require.NoError(t, err)
	names := make(map[string]bool, len(families))
	for _, family := range families {
		names[family.GetName()] = true
	}
	for _, name := range []string{
		"sparkops_event_outbox_pending",
		"sparkops_event_outbox_oldest_seconds",
		"sparkops_event_publish_total",
		"sparkops_event_dispatcher_error_total",
		"sparkops_event_consume_total",
		"sparkops_event_retry_total",
		"sparkops_event_dlq_total",
		"sparkops_event_inbox_duplicate_total",
		"sparkops_operation_outbox_pending",
		"sparkops_operation_outbox_oldest_seconds",
		"sparkops_operation_dispatcher_error_total",
	} {
		require.Truef(t, names[name], "metric %s was not registered", name)
	}
}
