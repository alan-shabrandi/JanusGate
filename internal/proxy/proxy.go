package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"janusgate/internal/circuitbreaker"
)

type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

func NewProxy(targetURL string, cb *circuitbreaker.CircuitBreaker) (http.Handler, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	if cb != nil {
		proxy.Transport = NewCircuitBreakerTransport(http.DefaultTransport, cb)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(r.Context().Err(), context.DeadlineExceeded) {
			w.WriteHeader(http.StatusGatewayTimeout)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Error:     "Gateway Timeout",
				Message:   "Upstream service failed to respond within the configured timeout duration.",
				Path:      r.URL.Path,
				Code:      http.StatusGatewayTimeout,
				Timestamp: time.Now().UTC(),
			})
			return
		}

		if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Error:     "Service Unavailable",
				Message:   "Upstream service is temporarily unavailable due to high error rates (Circuit Breaker Open).",
				Path:      r.URL.Path,
				Code:      http.StatusServiceUnavailable,
				Timestamp: time.Now().UTC(),
			})
			return
		}

		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error:     "Bad Gateway",
			Message:   fmt.Sprintf("Failed to reach upstream server: %v", err),
			Path:      r.URL.Path,
			Code:      http.StatusBadGateway,
			Timestamp: time.Now().UTC(),
		})
	}

	return proxy, nil
}

func StripPrefix(prefix string, h http.Handler) http.Handler {
	if prefix == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, prefix)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		r.URL.Path = p
		h.ServeHTTP(w, r)
	})
}
