package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	globalMetrics *Metrics
	once          sync.Once
)

type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
}

func Init(reg prometheus.Registerer) *Metrics {
	once.Do(func() {
		if reg == nil {
			reg = prometheus.DefaultRegisterer
		}

		factory := promauto.With(reg)

		globalMetrics = &Metrics{
			RequestsTotal: factory.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "janusgate",
					Subsystem: "http",
					Name:      "requests_total",
					Help:      "Total number of HTTP requests processed by JanusGate.",
				},
				[]string{"method", "path", "status"},
			),

			RequestDuration: factory.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: "janusgate",
					Subsystem: "http",
					Name:      "request_duration_seconds",
					Help:      "HTTP request latency distribution in seconds.",
					Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				},
				[]string{"method", "path"},
			),
		}
	})

	return globalMetrics
}

func Get() *Metrics {
	if globalMetrics == nil {
		return Init(prometheus.DefaultRegisterer)
	}
	return globalMetrics
}

func (m *Metrics) IncRequestsTotal(method, path string, statusCode int) {
	if m == nil || m.RequestsTotal == nil {
		return
	}
	statusStr := strconv.Itoa(statusCode)
	m.RequestsTotal.WithLabelValues(method, sanitizePathLabel(path), statusStr).Inc()
}

func (m *Metrics) ObserveRequestDuration(method, path string, duration time.Duration) {
	if m == nil || m.RequestDuration == nil {
		return
	}
	m.RequestDuration.WithLabelValues(method, sanitizePathLabel(path)).Observe(duration.Seconds())
}

func sanitizePathLabel(path string) string {
	if path == "" {
		return "/"
	}
	return path
}
