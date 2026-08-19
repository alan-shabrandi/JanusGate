package circuitbreaker

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type Config struct {
	Name               string
	MaxRequests        uint32
	Interval           time.Duration
	Timeout            time.Duration
	MinRequestsToTrip  uint32
	FailureRatioToTrip float64
}

type Transport struct {
	cb   *gobreaker.CircuitBreaker
	next http.RoundTripper
}

func NewTransport(cfg Config, next http.RoundTripper) *Transport {
	if cfg.Interval == 0 {
		cfg.Interval = 60 * time.Second
	}

	st := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < cfg.MinRequestsToTrip {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= cfg.FailureRatioToTrip
		},

		IsSuccessful: func(err error) bool {
			return err == nil
		},

		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			slog.Warn("circuit breaker state changed",
				"name", name,
				"from", from.String(),
				"to", to.String(),
			)
		},
	}

	return &Transport{
		cb:   gobreaker.NewCircuitBreaker(st),
		next: next,
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var httpResp *http.Response

	_, err := t.cb.Execute(func() (interface{}, error) {
		//nolint:bodyclose // The body is intentionally passed to the caller via httpResp to close.
		resp, err := t.next.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		httpResp = resp

		if resp.StatusCode >= http.StatusInternalServerError {
			return nil, errors.New("upstream 5xx")
		}

		return nil, nil
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, ErrCircuitOpen
		}

		if httpResp != nil {
			return httpResp, nil
		}

		return nil, err
	}

	return httpResp, nil
}
