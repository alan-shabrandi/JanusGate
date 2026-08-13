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
}

func (rw *responseWriterDelegator) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrappedWriter := &responseWriterDelegator{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrappedWriter, r)

		duration := time.Since(start)
		clientIP := extractClientIP(r)

		slog.Info("HTTP Request",
			slog.String("client_ip", clientIP),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", wrappedWriter.statusCode),
			slog.Duration("duration", duration),
		)
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
