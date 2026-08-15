package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriterDelegator struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriterDelegator) WriteHeader(code int) {
	if rw.statusCode == 0 {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriterDelegator) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

func (rw *responseWriterDelegator) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrappedWriter := &responseWriterDelegator{
			ResponseWriter: w,
			statusCode:     0,
		}

		next.ServeHTTP(wrappedWriter, r)

		latency := time.Since(start)

		status := wrappedWriter.statusCode
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
