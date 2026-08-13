package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
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

	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read request body for retry: %w", err)
		}
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.config.Attempts; attempt++ {
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err = t.base.RoundTrip(req)

		shouldRetry := t.isRetryable(resp, err)

		if !shouldRetry || attempt == t.config.Attempts {
			break
		}

		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		backoffDuration := t.calculateBackoff(attempt)

		slog.Warn("Retrying request to upstream",
			"path", req.URL.Path,
			"attempt", attempt+1,
			"max_attempts", t.config.Attempts,
			"backoff", backoffDuration.String(),
			"error", err,
		)

		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(backoffDuration):
		}
	}

	return resp, err
}

func (t *RetryTransport) isRetryable(resp *http.Response, err error) bool {
	if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		return false
	}

	if err != nil {
		return true
	}

	if resp != nil && resp.StatusCode >= http.StatusInternalServerError {
		return true
	}

	return false
}

func (t *RetryTransport) calculateBackoff(attempt int) time.Duration {
	floatInterval := float64(t.config.InitialInterval)
	backoff := floatInterval * math.Pow(2, float64(attempt))

	duration := time.Duration(backoff)
	if duration > t.config.MaxInterval {
		return t.config.MaxInterval
	}

	return duration
}
