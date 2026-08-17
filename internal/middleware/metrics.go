package middleware

import (
	"net/http"
	"time"

	"janusgate/internal/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

func Metrics(m *metrics.Metrics) Middleware {
	if m == nil {
		m = metrics.Get()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.IncActiveRequests()
			defer m.DecActiveRequests()

			start := time.Now()
			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			path := r.URL.Path

			m.IncRequestsTotal(r.Method, path, rw.statusCode)
			m.ObserveRequestDuration(r.Method, path, duration)
		})
	}
}
