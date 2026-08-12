package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func NewProxy(targetURL string) (*httputil.ReverseProxy, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL (%s): %w", targetURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parsedURL)

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy Error] failed to proxy request to %s: %v", targetURL, err)

		w.WriteHeader(http.StatusBadGateway)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error": "Bad Gateway", "message": "Upstream server is unreachable"}`))
	}

	return proxy, nil
}
