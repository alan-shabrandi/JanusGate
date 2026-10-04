package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"janusgate/internal/auth"
	"janusgate/internal/config"
	"janusgate/internal/middleware"
	"janusgate/internal/ratelimit"
	"janusgate/internal/router"
	"janusgate/internal/upstream"
)

func doRequest(t *testing.T, req *http.Request) (int, string) {
	t.Helper()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	return resp.StatusCode, string(bodyBytes)
}

func TestCoreIntegration(t *testing.T) {
	env := SetupTestEnv(t)

	mockUserService := NewMockUpstream(t)
	mockPublicService := NewMockUpstream(t)

	jwtSecret := "test-secret-key-32-bytes-long-exact!!"
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 0},
		Redis: config.RedisConfig{
			Addr:     env.RedisAddr,
			Password: "",
			DB:       0,
		},
		Auth: config.AuthConfig{
			JWTSecret: jwtSecret,
		},
		Routes: []config.RouteConfig{
			{
				ID:           "users-route",
				PathPrefix:   "/api/v1/users",
				StripPrefix:  true,
				RequiresAuth: true,
				Timeout:      3 * time.Second,
				Upstreams: []config.UpstreamConfig{
					{URL: mockUserService.URL, Weight: 1},
				},
			},
			{
				ID:           "public-route",
				PathPrefix:   "/api/v1/public",
				StripPrefix:  false,
				RequiresAuth: false,
				Timeout:      3 * time.Second,
				Upstreams: []config.UpstreamConfig{
					{URL: mockPublicService.URL, Weight: 1},
				},
			},
		},
	}

	registry := upstream.NewRegistry()
	jwtMgr, err := auth.NewJWTManager(cfg.Auth.JWTSecret, "")
	if err != nil {
		t.Fatalf("Failed to init JWTManager: %v", err)
	}

	redisLimiter, err := ratelimit.NewRedisLimiter(cfg.Redis)
	if err != nil {
		t.Fatalf("Failed to init RedisLimiter: %v", err)
	}
	defer func() {
		_ = redisLimiter.Close()
	}()

	rt := router.NewRouter(cfg.Routes, jwtMgr, registry, redisLimiter)

	globalChain := middleware.New(
		middleware.Recovery,
		middleware.RequestID(),
		middleware.RateLimit(redisLimiter, 5, time.Minute),
	)
	handler := globalChain.Then(rt)

	gwServer := httptest.NewServer(handler)
	defer gwServer.Close()

	validToken, err := jwtMgr.GenerateToken(context.Background(), "user-123", "admin", []string{"admin"}, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate test JWT token: %v", err)
	}

	t.Run("Auth - Request without token should return 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", gwServer.URL+"/api/v1/users/profile", nil)
		statusCode, _ := doRequest(t, req)

		if statusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got %d", statusCode)
		}
	})

	t.Run("Auth - Public route without token should return 200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", gwServer.URL+"/api/v1/public/ping", nil)
		statusCode, _ := doRequest(t, req)

		if statusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", statusCode)
		}
	})

	t.Run("Routing - Strip Prefix verification", func(t *testing.T) {
		req, _ := http.NewRequest("GET", gwServer.URL+"/api/v1/users/profile", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)

		statusCode, receivedPath := doRequest(t, req)

		if statusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", statusCode)
		}

		if receivedPath != "/profile" {
			t.Errorf("Expected upstream to receive path '/profile', got '%s'", receivedPath)
		}
	})

	t.Run("RateLimit - Exceeding limit should return 429", func(t *testing.T) {
		uniqueIP := "192.168.100.100"

		for i := 0; i < 5; i++ {
			req, _ := http.NewRequest("GET", gwServer.URL+"/api/v1/public/status", nil)
			req.Header.Set("X-Forwarded-For", uniqueIP)
			statusCode, _ := doRequest(t, req)

			if statusCode != http.StatusOK {
				t.Fatalf("Request %d expected 200 OK, got %d", i+1, statusCode)
			}
		}

		req, _ := http.NewRequest("GET", gwServer.URL+"/api/v1/public/status", nil)
		req.Header.Set("X-Forwarded-For", uniqueIP)
		statusCode, _ := doRequest(t, req)

		if statusCode != http.StatusTooManyRequests {
			t.Errorf("Expected 429 Too Many Requests, got %d", statusCode)
		}
	})
}
