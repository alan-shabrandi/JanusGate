package proxy

import (
	"net/http"

	"janusgate/internal/circuitbreaker"
)

type CircuitBreakerTransport struct {
	base http.RoundTripper
	cb   *circuitbreaker.CircuitBreaker
}

func NewCircuitBreakerTransport(base http.RoundTripper, cb *circuitbreaker.CircuitBreaker) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &CircuitBreakerTransport{
		base: base,
		cb:   cb,
	}
}

func (t *CircuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.cb.Execute(req, t.base)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
