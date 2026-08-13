package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"janusgate/internal/circuitbreaker"
)

type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// NewProxy یک ReverseProxy مجهز به CircuitBreaker و ErrorHandler سفارشی می‌سازد
func NewProxy(targetURL string, cb *circuitbreaker.CircuitBreaker) (http.Handler, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// ۱. تزریق RoundTripper سفارشی به پروکسی
	if cb != nil {
		proxy.Transport = NewCircuitBreakerTransport(http.DefaultTransport, cb)
	}

	// ۲. مدیریت خطاهای لایه شبکه و Circuit Breaker
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")

		// اگر مدار Open باشد
		if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Error:     "Service Unavailable",
				Message:   "Upstream service is temporarily unavailable due to high error rates (Circuit Breaker Open).",
				Path:      r.URL.Path,
				Code:      http.StatusServiceUnavailable,
				Timestamp: time.Now().UTC(),
			})
			return
		}

		// سایر خطاهای ارتباط با سرویس مقصد (مثل Down بودن سرور)
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error:     "Bad Gateway",
			Message:   fmt.Sprintf("Failed to reach upstream server: %v", err),
			Path:      r.URL.Path,
			Code:      http.StatusBadGateway,
			Timestamp: time.Now().UTC(),
		})
	}

	return proxy, nil
}

// StripPrefix میدلور حذف پیشوند مسیر
func StripPrefix(prefix string, h http.Handler) http.Handler {
	if prefix == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, prefix)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		r.URL.Path = p
		h.ServeHTTP(w, r)
	})
}
