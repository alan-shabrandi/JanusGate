package circuitbreaker

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

func New(name string, maxRequests uint32, timeout time.Duration) *CircuitBreaker {
	st := gobreaker.Settings{
		Name:        name,
		MaxRequests: maxRequests,
		Interval:    0,
		Timeout:     timeout,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.6
		},

		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			slog.Warn("Circuit Breaker state changed",
				"name", name,
				"from", from.String(),
				"to", to.String(),
			)
		},
	}

	return &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(st),
	}
}

func (cb *CircuitBreaker) Execute(req *http.Request, next http.RoundTripper) (*http.Response, error) {
	result, err := cb.cb.Execute(func() (interface{}, error) {
		resp, err := next.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= http.StatusInternalServerError {
			return resp, fmt.Errorf("upstream returned server error status: %d", resp.StatusCode)
		}

		return resp, nil
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, ErrCircuitOpen
		}
	}

	if resp, ok := result.(*http.Response); ok {
		return resp, nil
	}

	return nil, err
}
