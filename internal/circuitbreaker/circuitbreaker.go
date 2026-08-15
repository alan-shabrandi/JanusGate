package circuitbreaker

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
	errUpstream5xx = errors.New("upstream returned 5xx status")
)

type Config struct {
	Name               string
	MaxRequests        uint32
	Timeout            time.Duration
	MinRequestsToTrip  uint32
	FailureRatioToTrip float64
}
type Transport struct {
	cb   *gobreaker.CircuitBreaker
	next http.RoundTripper
}

func NewTransport(cfg Config, next http.RoundTripper) *Transport {
	st := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    0,
		Timeout:     cfg.Timeout,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= cfg.MinRequestsToTrip && failureRatio >= cfg.FailureRatioToTrip
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
	result, err := t.cb.Execute(func() (interface{}, error) {
		resp, err := t.next.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= http.StatusInternalServerError {
			return resp, errUpstream5xx
		}

		return resp, nil
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, ErrCircuitOpen
		}

		if errors.Is(err, errUpstream5xx) {
			return result.(*http.Response), nil
		}

		return nil, err
	}

	return result.(*http.Response), nil
}
