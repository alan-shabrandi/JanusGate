package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"janusgate/internal/auth"
	"janusgate/internal/ratelimit"
)

func RateLimit(limiter ratelimit.RateLimiter, limit int, window time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			var key string
			clientIP := extractClientIP(r)

			if userID, ok := auth.GetUserID(r.Context()); ok && userID != "" {
				key = "user:" + userID
			} else {
				key = "ip:" + clientIP
			}

			allowed, remaining, err := limiter.Allow(r.Context(), key, limit, window)
			if err != nil {
				slog.Error("Rate limiter error, allowing request (Fail-Open)",
					"error", err.Error(),
					"key", key,
					"client_ip", clientIP,
				)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				slog.Warn("Rate limit exceeded",
					"key", key,
					"client_ip", clientIP,
					"path", r.URL.Path,
				)

				retryAfterSeconds := int(window.Seconds())
				if retryAfterSeconds <= 0 {
					retryAfterSeconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))

				WriteJSONError(w, r, http.StatusTooManyRequests, "Rate limit exceeded. Please slow down and try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
