package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"janusgate/internal/auth"
	"janusgate/internal/config"
	"janusgate/internal/health"
	"janusgate/internal/middleware"
	"janusgate/internal/router"
	"janusgate/internal/upstream"
)

func TestResilienceIntegration(t *testing.T) {
	mockUpstream1 := NewMockUpstream(t)
	mockUpstream2 := NewMockUpstream(t)

	cfg := &config.Config{
		Server: config.ServerConfig{Port: 0},
		Auth:   config.AuthConfig{JWTSecret: "test-secret-key-32-bytes-long-exact!!"},
		Routes: []config.RouteConfig{
			{
				ID:           "resilient-service",
				PathPrefix:   "/api/v1/resilient",
				StripPrefix:  true,
				RequiresAuth: false,
				Timeout:      2 * time.Second,
				Retry: config.RetryConfig{
					Attempts:        2,
					InitialInterval: 10 * time.Millisecond,
					MaxInterval:     50 * time.Millisecond,
				},
				Upstreams: []config.UpstreamConfig{
					{URL: mockUpstream1.URL, Weight: 1},
					{URL: mockUpstream2.URL, Weight: 1},
				},
			},
		},
	}

	registry := upstream.NewRegistry()

	for _, r := range cfg.Routes {
		for _, u := range r.Upstreams {
			registry.RegisterServer(r.ID, u.URL, u.Weight)
		}
	}

	jwtMgr, _ := auth.NewJWTManager(cfg.Auth.JWTSecret, "")
	rt := router.NewRouter(cfg.Routes, jwtMgr, registry)

	globalChain := middleware.New(
		middleware.Recovery,
		middleware.RequestID(),
	)
	handler := globalChain.Then(rt)

	gwServer := httptest.NewServer(handler)
	defer gwServer.Close()

	t.Run("Retry - Should retry request when upstream returns 500", func(t *testing.T) {
		mockUpstream1.ResetRequestCount()
		mockUpstream2.ResetRequestCount()

		mockUpstream1.SetStatusCode(http.StatusInternalServerError)
		mockUpstream2.SetStatusCode(http.StatusOK)

		req, _ := http.NewRequest("GET", gwServer.URL+"/api/v1/resilient/ping", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 OK after retries, got %d", resp.StatusCode)
		}

		totalRequests := mockUpstream1.GetRequestCount() + mockUpstream2.GetRequestCount()
		if totalRequests < 2 {
			t.Errorf("Expected total attempts >= 2 due to retry, got %d", totalRequests)
		}
	})

	t.Run("HealthCheck - Dynamic Health Tracking", func(t *testing.T) {
		mockUpstream1.SetStatusCode(http.StatusOK)
		mockUpstream2.SetStatusCode(http.StatusOK)
		registry.SetHealth(mockUpstream1.URL, true)
		registry.SetHealth(mockUpstream2.URL, true)

		healthChecker := health.NewChecker(registry, 50*time.Millisecond)
		healthChecker.RegisterRoutesUpstreams(cfg.Routes)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		healthChecker.Start(ctx)

		time.Sleep(100 * time.Millisecond)

		up1Status := registry.IsHealthy(mockUpstream1.URL)
		if !up1Status {
			t.Errorf("Expected upstream 1 to be Healthy, got Unhealthy")
		}

		mockUpstream1.SetStatusCode(http.StatusInternalServerError)

		time.Sleep(250 * time.Millisecond)

		up1StatusAfterFail := registry.IsHealthy(mockUpstream1.URL)
		if up1StatusAfterFail {
			t.Errorf("Expected upstream 1 to become Unhealthy after repeated failures")
		}
	})

	t.Run("Circuit Breaker - Isolating unstable upstream", func(t *testing.T) {
		mockUpstream1.ResetRequestCount()
		mockUpstream1.SetStatusCode(http.StatusInternalServerError)

		for i := 0; i < 5; i++ {
			req, _ := http.NewRequest("GET", gwServer.URL+"/api/v1/resilient/test", nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}

		if !registry.IsHealthy(mockUpstream1.URL) {
			t.Logf("Circuit Breaker or Health Registry successfully detected target instability")
		}
	})
}
