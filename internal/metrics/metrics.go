package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	ActiveConnections    *prometheus.GaugeVec
	UpstreamStatus       *prometheus.GaugeVec
	UpstreamRequestCount *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	factory := promauto.With(reg)

	return &Metrics{
		HTTPRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "janusgate",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests processed by the gateway.",
			},
			[]string{"method", "path", "status"},
		),

		HTTPRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "janusgate",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request latency distributions in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),

		ActiveConnections: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "janusgate",
				Subsystem: "http",
				Name:      "active_connections",
				Help:      "Current number of active connections per upstream server.",
			},
			[]string{"target_url"},
		),

		UpstreamStatus: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "janusgate",
				Subsystem: "upstream",
				Name:      "health_status",
				Help:      "Health status of upstream targets (1 for healthy, 0 for unhealthy).",
			},
			[]string{"route_id", "target_url"},
		),

		UpstreamRequestCount: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "janusgate",
				Subsystem: "upstream",
				Name:      "requests_total",
				Help:      "Total number of requests forwarded to upstream targets.",
			},
			[]string{"route_id", "target_url", "status"},
		),
	}
}
