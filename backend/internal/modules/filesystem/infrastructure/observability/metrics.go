package observability

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "sparkops"
	subsystem = "filesystem"
)

var (
	TransferActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "transfer_active",
		Help: "Current active filesystem transfers.",
	}, []string{"protocol"})
	TransferBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "transfer_bytes_total",
		Help: "Total bytes transferred by the filesystem data plane.",
	}, []string{"direction", "protocol"})
	TransferOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "transfer_operations_total",
		Help: "Total filesystem transfer operations by result.",
	}, []string{"protocol", "result"})
	TransferFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "transfer_failures_total",
		Help: "Total failed filesystem transfers.",
	}, []string{"reason"})
	TransferStateDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "transfer_state_duration_seconds",
		Help:    "Time spent by transfers in each state.",
		Buckets: prometheus.ExponentialBuckets(0.1, 5, 10),
	}, []string{"state"})
	ScannerLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "scanner_latency_seconds",
		Help:    "Malware scanner request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"result"})
	ScannerRejections = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "scanner_rejections_total",
		Help: "Total files rejected by malware scanning.",
	}, []string{"reason"})
	QueueLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "queue_lag_seconds",
		Help: "Age of the oldest pending filesystem queue item.",
	}, []string{"queue"})
	ProviderErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "provider_errors_total",
		Help: "Total storage provider operation errors.",
	}, []string{"provider", "operation"})
	ProviderLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "provider_latency_seconds",
		Help:    "Storage provider operation latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider", "operation"})
	QuotaReservedBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "quota_reserved_bytes",
		Help: "Bytes reserved by active filesystem transfers.",
	}, []string{"workspace"})
	QuotaUsedBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "quota_used_bytes",
		Help: "Bytes committed to filesystem quota.",
	}, []string{"workspace"})
	ReplicaInconsistency = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "replica_inconsistency",
		Help: "Number of blobs whose replicas disagree with desired state.",
	})
	GCBacklog = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "gc_backlog",
		Help: "Number of blobs awaiting garbage collection.",
	})
	ReconcileBacklog = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: subsystem, Name: "reconcile_backlog",
		Help: "Number of filesystem records awaiting reconciliation.",
	})
)

var collectors = []prometheus.Collector{
	TransferActive, TransferBytes, TransferOperations, TransferFailures, TransferStateDuration,
	ScannerLatency, ScannerRejections, QueueLag, ProviderErrors, ProviderLatency,
	QuotaReservedBytes, QuotaUsedBytes, ReplicaInconsistency, GCBacklog, ReconcileBacklog,
}

// Register adds filesystem collectors to a registry. Re-registering the same
// collectors is safe so API and worker processes can share bootstrap code.
func Register(registry prometheus.Registerer) error {
	initializeMetricFamilies()
	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			var already prometheus.AlreadyRegisteredError
			if errors.As(err, &already) && already.ExistingCollector == collector {
				continue
			}
			return err
		}
	}
	return nil
}

func initializeMetricFamilies() {
	TransferActive.WithLabelValues("unknown")
	TransferBytes.WithLabelValues("unknown", "unknown")
	TransferOperations.WithLabelValues("unknown", "unknown")
	TransferFailures.WithLabelValues("unknown")
	TransferStateDuration.WithLabelValues("unknown")
	ScannerLatency.WithLabelValues("unknown")
	ScannerRejections.WithLabelValues("unknown")
	QueueLag.WithLabelValues("unknown")
	ProviderErrors.WithLabelValues("unknown", "unknown")
	ProviderLatency.WithLabelValues("unknown", "unknown")
	QuotaReservedBytes.WithLabelValues("unknown")
	QuotaUsedBytes.WithLabelValues("unknown")
}
