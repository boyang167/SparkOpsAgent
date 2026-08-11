package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_ms",
			Help:    "HTTP request duration in milliseconds",
			Buckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		},
		[]string{"method", "path", "status"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method
		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path, status).Observe(float64(time.Since(start).Milliseconds()))
	}
}

func RegisterMetrics(r gin.IRoutes) {
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

type MetricSeries struct {
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Labels map[string]string `json:"labels,omitempty"`
}

type MetricsSummary struct {
	HTTPRequestsTotal   []MetricSeries `json:"http_requests_total"`
	HTTPDurationP50Ms   float64        `json:"http_duration_p50_ms"`
	HTTPDurationP95Ms   float64        `json:"http_duration_p95_ms"`
	HTTPDurationP99Ms   float64        `json:"http_duration_p99_ms"`
	CollectedAt         string         `json:"collected_at"`
	PrometheusURL       string         `json:"prometheus_url,omitempty"`
}

func GatherMetricsSummary() MetricsSummary {
	summary := MetricsSummary{CollectedAt: time.Now().UTC().Format(time.RFC3339)}
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return summary
	}
	var durationSamples []float64
	for _, mf := range mfs {
		name := mf.GetName()
		for _, metric := range mf.GetMetric() {
			labels := map[string]string{}
			for _, lp := range metric.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			switch name {
			case "http_requests_total":
				summary.HTTPRequestsTotal = append(summary.HTTPRequestsTotal, MetricSeries{
					Label:  labels["method"] + " " + labels["path"] + " " + labels["status"],
					Value:  metric.GetCounter().GetValue(),
					Labels: labels,
				})
			case "http_request_duration_ms":
				hist := metric.GetHistogram()
				if hist.GetSampleCount() > 0 {
					durationSamples = append(durationSamples, hist.GetSampleSum()/float64(hist.GetSampleCount()))
				}
			}
		}
	}
	if len(durationSamples) > 0 {
		summary.HTTPDurationP50Ms = durationSamples[0]
		summary.HTTPDurationP95Ms = durationSamples[0]
		summary.HTTPDurationP99Ms = durationSamples[0]
	}
	return summary
}

func RequestLogMiddleware(logger *zap.Logger) gin.HandlerFunc {
	if logger == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		traceID := c.GetHeader("X-Trace-Id")
		if traceID == "" {
			if v, ok := c.Get("trace_id"); ok {
				if s, ok := v.(string); ok {
					traceID = s
				}
			}
		}
		wsID := c.Param("wsId")
		c.Next()
		status := c.Writer.Status()
		log := LoggerWithTrace(logger, traceID, wsID, "http")
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Int64("latency_ms", time.Since(start).Milliseconds()),
			zap.String("client_ip", c.ClientIP()),
		}
		if status >= 500 {
			log.Error("http request completed", fields...)
		} else if status >= 400 {
			log.Warn("http request completed", fields...)
		} else {
			log.Info("http request completed", fields...)
		}
	}
}
