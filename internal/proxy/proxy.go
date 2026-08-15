package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"janusgate/internal/circuitbreaker"
	"janusgate/internal/config"
)

func NewProxy(targetURL string, cb *circuitbreaker.Transport, retryCfg config.RetryConfig) (http.Handler, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	var transport http.RoundTripper = http.DefaultTransport

	if retryCfg.Attempts > 0 {
		transport = NewRetryTransport(transport, retryCfg)
	}

	if cb != nil {
		transport = cb
	}

	proxy.Transport = transport

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(r.Context().Err(), context.DeadlineExceeded) {
			w.WriteHeader(http.StatusGatewayTimeout)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Error:     "Gateway Timeout",
				Message:   "Upstream service failed to respond within configured timeout.",
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
				Message:   "Upstream service is temporarily unavailable due to high failure rate (Circuit Breaker Open).",
				Path:      r.URL.Path,
				Code:      http.StatusServiceUnavailable,
				Timestamp: time.Now().UTC(),
			})
			return
		}

		slog.Error("Upstream connection failed", "target", targetURL, "path", r.URL.Path, "error", err)

		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error:     "Bad Gateway",
			Message:   "Failed to reach upstream server.",
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
		r.URL.RawPath = ""

		h.ServeHTTP(w, r)
	})
}
