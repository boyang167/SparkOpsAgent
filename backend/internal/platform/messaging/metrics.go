package messaging

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	OutboxPending          prometheus.Gauge
	OutboxOldestSeconds    prometheus.Gauge
	EventPublish           *prometheus.CounterVec
	EventConsume           *prometheus.CounterVec
	EventRetry             *prometheus.CounterVec
	EventDLQ               *prometheus.CounterVec
	InboxDuplicate         *prometheus.CounterVec
	OperationPending       prometheus.Gauge
	OperationOldestSeconds prometheus.Gauge
}

var metricsRegistrations = struct {
	sync.Mutex
	values map[prometheus.Registerer]*Metrics
}{values: make(map[prometheus.Registerer]*Metrics)}

func NewMetrics(registerer prometheus.Registerer) (*Metrics, error) {
	if registerer == nil {
		return nil, fmt.Errorf("prometheus registerer is required")
	}
	metricsRegistrations.Lock()
	defer metricsRegistrations.Unlock()
	if registered := metricsRegistrations.values[registerer]; registered != nil {
		return registered, nil
	}

	metrics := &Metrics{
		OutboxPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sparkops_event_outbox_pending",
			Help: "Number of event outbox records waiting to be published.",
		}),
		OutboxOldestSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sparkops_event_outbox_oldest_seconds",
			Help: "Age in seconds of the oldest pending event outbox record.",
		}),
		EventPublish: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sparkops_event_publish_total",
			Help: "Total event publish attempts by result.",
		}, []string{"result"}),
		EventConsume: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sparkops_event_consume_total",
			Help: "Total event consumption outcomes.",
		}, []string{"subscriber", "result"}),
		EventRetry: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sparkops_event_retry_total",
			Help: "Total event retries by subscriber and delay.",
		}, []string{"subscriber", "delay"}),
		EventDLQ: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sparkops_event_dlq_total",
			Help: "Total events routed to a subscriber dead-letter queue.",
		}, []string{"subscriber"}),
		InboxDuplicate: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sparkops_event_inbox_duplicate_total",
			Help: "Total duplicate inbox deliveries by subscriber.",
		}, []string{"subscriber"}),
		OperationPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sparkops_operation_outbox_pending",
			Help: "Number of pending operation outbox records.",
		}),
		OperationOldestSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sparkops_operation_outbox_oldest_seconds",
			Help: "Age in seconds of the oldest pending operation outbox record.",
		}),
	}
	collectors := []prometheus.Collector{
		metrics.OutboxPending,
		metrics.OutboxOldestSeconds,
		metrics.EventPublish,
		metrics.EventConsume,
		metrics.EventRetry,
		metrics.EventDLQ,
		metrics.InboxDuplicate,
		metrics.OperationPending,
		metrics.OperationOldestSeconds,
	}
	registered := make([]prometheus.Collector, 0, len(collectors))
	for _, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			for _, previous := range registered {
				registerer.Unregister(previous)
			}
			return nil, err
		}
		registered = append(registered, collector)
	}
	metricsRegistrations.values[registerer] = metrics
	return metrics, nil
}
