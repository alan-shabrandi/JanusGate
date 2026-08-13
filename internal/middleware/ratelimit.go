package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"janusgate/internal/ratelimit"
)

type rateLimitErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

func RateLimit(limiter ratelimit.RateLimiter, limit int, window time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := extractClientIP(r)
			key := "ip:" + clientIP

			allowed, remaining, err := limiter.Allow(r.Context(), key, limit, window)
			if err != nil {
				slog.Error("Rate limiter error, allowing request (Fail-Open)",
					"error", err,
					"client_ip", clientIP,
				)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				slog.Warn("Rate limit exceeded for client",
					"client_ip", clientIP,
					"path", r.URL.Path,
				)

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)

				resp := rateLimitErrorResponse{
					Error:     "Too Many Requests",
					Message:   "Rate limit exceeded. Please slow down and try again later.",
					Path:      r.URL.Path,
					Code:      http.StatusTooManyRequests,
					Timestamp: time.Now().UTC(),
				}

				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
