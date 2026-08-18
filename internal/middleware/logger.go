package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := getRecorder(w)
		next.ServeHTTP(rw, r)

		latency := time.Since(start)

		status := rw.StatusCode
		if status == 0 {
			status = http.StatusOK
		}

		clientIP := extractClientIP(r)

		fields := []any{
			slog.String("client_ip", clientIP),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.String("latency", latency.String()),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.Int64("bytes_written", rw.BytesWritten),
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
