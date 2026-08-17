package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"janusgate/internal/middleware"
)

var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func BenchmarkMiddleware_RequestID(b *testing.B) {
	handler := middleware.RequestID()(noopHandler)
	req := httptest.NewRequest("GET", "/api/v1/test", nil)

	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkMiddleware_RequestID_WithExistingHeader(b *testing.B) {
	handler := middleware.RequestID()(noopHandler)
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("X-Request-ID", "existing-client-uuid-12345")

	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkMiddleware_Recovery(b *testing.B) {
	handler := middleware.Recovery(noopHandler)
	req := httptest.NewRequest("GET", "/api/v1/test", nil)

	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkMiddleware_FullPipeline(b *testing.B) {
	chain := middleware.New(
		middleware.Recovery,
		middleware.RequestID(),
		middleware.Trace(),
	)
	handler := chain.Then(noopHandler)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)

	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
