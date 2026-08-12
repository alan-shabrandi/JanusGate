package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

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

	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Server")
		resp.Header.Del("X-Powered-By")
		resp.Header.Set("Via", "JanusGate")
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy Error] Target: %s | Client: %s | Error: %v", targetURL, r.RemoteAddr, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)

		errResp := ErrorResponse{
			Error:   "Bad Gateway",
			Message: "The upstream server is unreachable or offline.",
			Code:    http.StatusBadGateway,
		}

		_ = json.NewEncoder(w).Encode(errResp)
	}

	return proxy, nil
}

func StripPrefix(prefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if prefix == "" || prefix == "/" {
			next.ServeHTTP(w, r)
			return
		}

		p := strings.TrimPrefix(r.URL.Path, prefix)

		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}

		r.URL.Path = p
		r.URL.RawPath = p

		next.ServeHTTP(w, r)
	})
}
