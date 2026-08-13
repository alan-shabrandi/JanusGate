package proxy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"janusgate/internal/middleware"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

var optimizedTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          1000,
	MaxIdleConnsPerHost:   100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func NewProxy(targetURL string) (*httputil.ReverseProxy, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL (%s): %w", targetURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parsedURL)

	proxy.Transport = optimizedTransport

	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Host = parsedURL.Host

		clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			clientIP = req.RemoteAddr
		}

		req.Header.Set("X-Real-IP", clientIP)

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
		slog.Error("Proxy Error", "target", targetURL, "client", r.RemoteAddr, "error", err)

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

		r.URL.RawPath = ""

		next.ServeHTTP(w, r)
	})
}

func EnrichUpstreamHeaders(req *http.Request) {
	claims, ok := middleware.GetUserClaims(req.Context())
	if !ok || claims == nil {
		return
	}

	req.Header.Set("X-User-ID", claims.UserID)
	req.Header.Set("X-User-Name", claims.Username)

	if len(claims.Roles) > 0 {
		req.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
	}
}
