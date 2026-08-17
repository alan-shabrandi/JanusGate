package router_test

import (
	"net/http/httptest"
	"testing"

	"janusgate/internal/config"
	"janusgate/internal/router"
)

func setupBenchmarkRouter() router.Router {
	routes := []config.RouteConfig{
		{
			ID:          "exact-user",
			PathPrefix:  "/api/v1/users/profile",
			MatchType:   "exact",
			Methods:     []string{"GET"},
			Upstreams:   []config.UpstreamConfig{{URL: "http://localhost:8081"}},
			StripPrefix: false,
		},
		{
			ID:          "prefix-users",
			PathPrefix:  "/api/v1/users",
			MatchType:   "prefix",
			Methods:     []string{"GET", "POST"},
			Upstreams:   []config.UpstreamConfig{{URL: "http://localhost:8081"}},
			StripPrefix: true,
		},
		{
			ID:          "prefix-orders",
			PathPrefix:  "/api/v1/orders",
			MatchType:   "prefix",
			Methods:     []string{"GET"},
			Upstreams:   []config.UpstreamConfig{{URL: "http://localhost:8082"}},
			StripPrefix: true,
		},
	}

	return router.NewRouter(routes, nil, nil)
}

func BenchmarkRouter_ExactMatch(b *testing.B) {
	r := setupBenchmarkRouter()
	req := httptest.NewRequest("GET", "/api/v1/users/profile", nil)

	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_PrefixMatch(b *testing.B) {
	r := setupBenchmarkRouter()
	req := httptest.NewRequest("GET", "/api/v1/users/12345/details", nil)

	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_NotFound(b *testing.B) {
	r := setupBenchmarkRouter()
	req := httptest.NewRequest("GET", "/api/v1/non-existent-path", nil)

	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
