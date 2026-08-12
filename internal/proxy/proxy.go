package proxy

import (
	"fmt"
	"log"
	"net"
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

	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			clientIP = req.RemoteAddr
		}

		req.Header.Set("X-Real-IP", clientIP)

		if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
			req.Header.Set("X-Forwarded-For", prior+", "+clientIP)
		} else {
			req.Header.Set("X-Forwarded-For", clientIP)
		}

		if req.Host != "" {
			req.Header.Set("X-Forwarded-Host", req.Host)
		} else {
			req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
		}

		scheme := "http"
		if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		req.Header.Set("X-Forwarded-Proto", scheme)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy Error] failed to proxy request to %s: %v", targetURL, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error": "Bad Gateway", "message": "Upstream server is unreachable"}`))
	}

	return proxy, nil
}
