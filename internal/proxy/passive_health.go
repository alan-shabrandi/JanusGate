package proxy

import (
	"net/http"

	"janusgate/internal/upstream"
)

type PassiveHealthTransport struct {
	base      http.RoundTripper
	targetURL string
	registry  *upstream.Registry
}

func NewPassiveHealthTransport(base http.RoundTripper, targetURL string, reg *upstream.Registry) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &PassiveHealthTransport{
		base:      base,
		targetURL: targetURL,
		registry:  reg,
	}
}

func (t *PassiveHealthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)

	if t.registry == nil {
		return resp, err
	}

	if err != nil {
		t.registry.SetHealth(t.targetURL, false)
		return resp, err
	}

	if resp != nil {
		if resp.StatusCode >= http.StatusInternalServerError {
			t.registry.SetHealth(t.targetURL, false)
		} else if resp.StatusCode < http.StatusBadRequest {
			t.registry.SetHealth(t.targetURL, true)
		}
	}

	return resp, err
}
