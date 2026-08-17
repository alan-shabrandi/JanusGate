package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"janusgate/internal/circuitbreaker"
	"janusgate/internal/config"
	"janusgate/internal/upstream"
)

type TracingTransport struct {
	Base http.RoundTripper
}

func NewTracingTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &TracingTransport{Base: base}
}

func (t *TracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqClone := req.Clone(req.Context())
	otel.GetTextMapPropagator().Inject(reqClone.Context(), propagation.HeaderCarrier(reqClone.Header))
	return t.Base.RoundTrip(reqClone)
}

func NewProxy(targetURL string, cbCfg *circuitbreaker.Config, retryCfg config.RetryConfig, reg *upstream.Registry) (http.Handler, error) {
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

	var baseTransport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	chainedTransport := baseTransport

	chainedTransport = NewTracingTransport(chainedTransport)

	if reg != nil {
		chainedTransport = NewPassiveHealthTransport(chainedTransport, targetURL, reg)
	}

	if cbCfg != nil {
		chainedTransport = circuitbreaker.NewTransport(*cbCfg, chainedTransport)
	}

	if retryCfg.Attempts > 0 {
		chainedTransport = NewRetryTransport(chainedTransport, retryCfg)
	}

	proxy.Transport = chainedTransport

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")

		if reg != nil {
			reg.SetHealth(targetURL, false)
		}

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(r.Context().Err(), context.DeadlineExceeded) {
			writeError(w, r, http.StatusGatewayTimeout, "Gateway Timeout", "Upstream service failed to respond within configured timeout.")
			return
		}

		if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
			writeError(w, r, http.StatusServiceUnavailable, "Service Unavailable", "Upstream service is temporarily unavailable due to high failure rate.")
			return
		}

		slog.Error("Upstream connection failed", "target", targetURL, "path", r.URL.Path, "error", err)
		writeError(w, r, http.StatusBadGateway, "Bad Gateway", "Failed to reach upstream server.")
	}

	return proxy, nil
}

func writeError(w http.ResponseWriter, r *http.Request, code int, errType, message string) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:     errType,
		Message:   message,
		Path:      r.URL.Path,
		Code:      code,
		Timestamp: time.Now().UTC(),
	})
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
