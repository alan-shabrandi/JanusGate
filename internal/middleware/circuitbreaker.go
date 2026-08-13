package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"janusgate/internal/circuitbreaker"
)

func CircuitBreaker(cb *circuitbreaker.CircuitBreaker) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

func respondCircuitOpen(w http.ResponseWriter, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)

	resp := map[string]interface{}{
		"error":     "Service Unavailable",
		"message":   "Upstream service is temporarily unavailable due to high error rates (Circuit Breaker Open).",
		"path":      path,
		"code":      http.StatusServiceUnavailable,
		"timestamp": time.Now().UTC(),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
