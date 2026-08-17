//internal/metrics/metrics.go

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
	ActiveRequests  prometheus.Gauge
	UpstreamHealthy *prometheus.GaugeVec
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
				[]string{"method", "route_id", "status"},
			),

			RequestDuration: factory.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: "janusgate",
					Subsystem: "http",
					Name:      "request_duration_seconds",
					Help:      "HTTP request latency distribution in seconds.",
					Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				},
				[]string{"method", "route_id"},
			),

			ActiveRequests: factory.NewGauge(
				prometheus.GaugeOpts{
					Namespace: "janusgate",
					Subsystem: "http",
					Name:      "active_requests",
					Help:      "Current number of active requests in progress.",
				},
			),

			UpstreamHealthy: factory.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: "janusgate",
					Subsystem: "upstream",
					Name:      "healthy",
					Help:      "Health status of upstreams (1 for healthy/up, 0 for unhealthy/down).",
				},
				[]string{"route_id", "target_url"},
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

func (m *Metrics) IncRequestsTotal(method, routeID string, statusCode int) {
	if m == nil || m.RequestsTotal == nil {
		return
	}
	if routeID == "" {
		routeID = "unknown"
	}
	statusStr := strconv.Itoa(statusCode)
	m.RequestsTotal.WithLabelValues(method, routeID, statusStr).Inc()
}

func (m *Metrics) ObserveRequestDuration(method, routeID string, duration time.Duration) {
	if m == nil || m.RequestDuration == nil {
		return
	}
	if routeID == "" {
		routeID = "unknown"
	}
	m.RequestDuration.WithLabelValues(method, routeID).Observe(duration.Seconds())
}

func (m *Metrics) IncActiveRequests() {
	if m == nil || m.ActiveRequests == nil {
		return
	}
	m.ActiveRequests.Inc()
}

func (m *Metrics) DecActiveRequests() {
	if m == nil || m.ActiveRequests == nil {
		return
	}
	m.ActiveRequests.Dec()
}

func (m *Metrics) SetUpstreamHealth(routeID, targetURL string, isHealthy bool) {
	if m == nil || m.UpstreamHealthy == nil {
		return
	}
	var val float64
	if isHealthy {
		val = 1.0
	}
	m.UpstreamHealthy.WithLabelValues(routeID, targetURL).Set(val)
}
