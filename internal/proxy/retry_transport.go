package proxy

import (
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"janusgate/internal/circuitbreaker"
	"janusgate/internal/config"
)

type RetryTransport struct {
	base   http.RoundTripper
	config config.RetryConfig
}

func NewRetryTransport(base http.RoundTripper, cfg config.RetryConfig) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RetryTransport{
		base:   base,
		config: cfg,
	}
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.config.Attempts <= 0 {
		return t.base.RoundTrip(req)
	}

	if req.Body != nil && req.GetBody == nil {
		return t.base.RoundTrip(req)
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.config.Attempts; attempt++ {
		ctx := req.Context()
		reqClone := req.Clone(ctx)

		if reqClone.Body != nil && req.GetBody != nil {
			reqClone.Body, err = req.GetBody()
			if err != nil {
				return nil, err
			}
		}

		resp, err = t.base.RoundTrip(reqClone)

		shouldRetry := t.isRetryable(reqClone.Method, resp, err)

		if !shouldRetry || attempt == t.config.Attempts {
			break
		}

		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}

		backoffDuration := t.calculateBackoff(attempt)

		slog.Warn("retrying request to upstream",
			"path", req.URL.Path,
			"method", req.Method,
			"attempt", attempt+1,
			"max_attempts", t.config.Attempts,
			"backoff", backoffDuration.String(),
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDuration):
		}
	}

	return resp, err
}

func (t *RetryTransport) isRetryable(method string, resp *http.Response, err error) bool {
	if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		return false
	}

	if !isIdempotent(method) {
		return false
	}

	if err != nil {
		return true
	}

	if resp != nil && (resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusGatewayTimeout) {
		return true
	}

	return false
}

func (t *RetryTransport) calculateBackoff(attempt int) time.Duration {
	floatInterval := float64(t.config.InitialInterval)
	backoff := floatInterval * float64(int(1)<<attempt)

	duration := time.Duration(backoff)
	if duration > t.config.MaxInterval {
		duration = t.config.MaxInterval
	}

	jitter := time.Duration(rand.Int63n(int64(duration)/10 + 1))
	return duration + jitter
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete, http.MethodTrace:
		return true
	default:
		return false
	}
}
