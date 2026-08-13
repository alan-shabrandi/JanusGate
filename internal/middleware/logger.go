package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type responseWriterDelegator struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriterDelegator) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterDelegator) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrappedWriter := &responseWriterDelegator{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrappedWriter, r)

		latency := time.Since(start)

		status := wrappedWriter.statusCode
		clientIP := extractClientIP(r)

		fields := []any{
			slog.String("client_ip", clientIP),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.String("latency", latency.String()),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.Int64("bytes_written", wrappedWriter.written),
		}

		switch {
		case status >= 500:
			slog.Error("HTTP Request Failed", fields...)
		case status >= 400:
			slog.Warn("HTTP Request Client Error", fields...)
		default:
			slog.Info("HTTP Request Handled", fields...)
		}
	})
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}
