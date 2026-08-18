package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"janusgate/internal/config"
	"janusgate/internal/router"
)

type benchResponseWriter struct {
	header http.Header
}

func (w *benchResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *benchResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (w *benchResponseWriter) WriteHeader(statusCode int) {}

func BenchmarkRouter_ServeHTTP_ExactMatch(b *testing.B) {
	routes := []config.RouteConfig{
		{
			ID:         "exact-route",
			PathPrefix: "/api/v1/status",
			MatchType:  "exact",
			Methods:    []string{http.MethodGet},
			Upstreams:  []config.UpstreamConfig{{URL: "http://127.0.0.1:8080"}},
		},
	}
	r := router.NewRouter(routes, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := &benchResponseWriter{}
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_ServeHTTP_PrefixMatch(b *testing.B) {
	routes := []config.RouteConfig{
		{
			ID:         "prefix-route",
			PathPrefix: "/api/v1/users",
			MatchType:  "prefix",
			Methods:    []string{http.MethodGet, http.MethodPost},
			Upstreams:  []config.UpstreamConfig{{URL: "http://127.0.0.1:8080"}},
		},
	}
	r := router.NewRouter(routes, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/12345/profile", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := &benchResponseWriter{}
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_ServeHTTP_NotFound(b *testing.B) {
	routes := []config.RouteConfig{
		{
			ID:         "exact-route",
			PathPrefix: "/api/v1/status",
			MatchType:  "exact",
			Methods:    []string{http.MethodGet},
			Upstreams:  []config.UpstreamConfig{{URL: "http://127.0.0.1:8080"}},
		},
	}
	r := router.NewRouter(routes, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := &benchResponseWriter{}
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_ServeHTTP_MethodNotAllowed(b *testing.B) {
	routes := []config.RouteConfig{
		{
			ID:         "exact-route",
			PathPrefix: "/api/v1/status",
			MatchType:  "exact",
			Methods:    []string{http.MethodPost},
			Upstreams:  []config.UpstreamConfig{{URL: "http://127.0.0.1:8080"}},
		},
	}
	r := router.NewRouter(routes, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := &benchResponseWriter{}
		r.ServeHTTP(w, req)
	}
}
