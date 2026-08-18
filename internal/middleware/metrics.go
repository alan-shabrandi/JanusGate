package middleware

import (
	"net/http"
	"time"

	"janusgate/internal/metrics"
)

func Metrics(m *metrics.Metrics) Middleware {
	if m == nil {
		m = metrics.Get()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.IncActiveRequests()
			defer m.DecActiveRequests()

			start := time.Now()

			rw := getRecorder(w)
			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			status := rw.StatusCode
			if status == 0 {
				status = http.StatusOK
			}

			pathPattern := r.Pattern
			if pathPattern == "" {
				pathPattern = "unknown_route"
			}

			m.IncRequestsTotal(r.Method, pathPattern, status)
			m.ObserveRequestDuration(r.Method, pathPattern, duration)
		})
	}
}
