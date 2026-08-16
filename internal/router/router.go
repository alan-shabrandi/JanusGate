package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"janusgate/internal/auth"
	"janusgate/internal/circuitbreaker"
	"janusgate/internal/config"
	"janusgate/internal/middleware"
	"janusgate/internal/proxy"
)

var (
	ErrRouteNotFound = errors.New("route not found")
	ErrInvalidPath   = errors.New("invalid route path")
)

type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Path      string    `json:"path"`
	Code      int       `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

type Router interface {
	LoadRoutes(routes []config.RouteConfig) error
	http.Handler
}

type routeEntry struct {
	config  config.RouteConfig
	handler http.Handler
}

type memoryRouter struct {
	routes   atomic.Pointer[[]routeEntry]
	tokenMgr auth.TokenManager
}

func NewRouter(routes []config.RouteConfig, tokenMgr auth.TokenManager) Router {
	r := &memoryRouter{
		tokenMgr: tokenMgr,
	}

	if err := r.LoadRoutes(routes); err != nil {
		slog.Error("Failed to load initial routes", "error", err)
	}

	return r
}

func (r *memoryRouter) LoadRoutes(routes []config.RouteConfig) error {
	newRoutes := make([]routeEntry, 0, len(routes))

	for _, route := range routes {
		if len(route.Upstreams) == 0 {
			slog.Warn("Route has no upstreams configured, skipping...", "route_id", route.ID)
			continue
		}

		if route.PathPrefix == "" {
			return fmt.Errorf("route %s has empty path_prefix", route.ID)
		}

		if route.MatchType == "" {
			route.MatchType = "prefix"
		}

		primaryUpstream := route.Upstreams[0].URL
		cbConfig := circuitbreaker.Config{
			Name:               route.ID,
			MaxRequests:        1,
			Timeout:            10 * time.Second,
			MinRequestsToTrip:  5,
			FailureRatioToTrip: 0.5,
		}

		revProxy, err := proxy.NewProxy(primaryUpstream, &cbConfig, route.Retry)
		if err != nil {
			return fmt.Errorf("failed to create proxy for route %s: %w", route.ID, err)
		}

		pipeline := middleware.New()

		pipeline = pipeline.Use(middleware.Timeout(route.Timeout))

		if route.RequiresAuth {
			if r.tokenMgr == nil {
				slog.Error("Route requires authentication but TokenManager is nil", "route_id", route.ID)
			} else {
				pipeline = pipeline.Use(middleware.Authenticate(r.tokenMgr))
				slog.Info("Route loaded [PRIVATE]", "id", route.ID, "path", route.PathPrefix, "target", primaryUpstream)
			}
		} else {
			slog.Info("Route loaded [PUBLIC]", "id", route.ID, "path", route.PathPrefix, "target", primaryUpstream)
		}

		var handler http.Handler = revProxy
		if route.StripPrefix {
			handler = proxy.StripPrefix(route.PathPrefix, handler)
		}

		finalHandler := pipeline.Then(handler)

		newRoutes = append(newRoutes, routeEntry{
			config:  route,
			handler: finalHandler,
		})
	}

	r.routes.Store(&newRoutes)

	return nil
}

func (r *memoryRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	routesPtr := r.routes.Load()
	if routesPtr == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Service Unavailable", "Gateway is initializing", req.URL.Path)
		return
	}
	currentRoutes := *routesPtr

	cleanPath := path.Clean(req.URL.Path)

	var matchedHandler http.Handler
	methodMatched := false
	pathMatched := false
	longestPrefix := -1

	for _, entry := range currentRoutes {
		if entry.config.MatchType == "exact" && entry.config.PathPrefix == cleanPath {
			pathMatched = true
			if isMethodAllowed(entry.config.Methods, req.Method) {
				matchedHandler = entry.handler
				methodMatched = true
				break
			}
		}
	}

	if matchedHandler == nil {
		for _, entry := range currentRoutes {
			if entry.config.MatchType == "prefix" {
				prefix := entry.config.PathPrefix
				if strings.HasPrefix(cleanPath, prefix) && (len(cleanPath) == len(prefix) || cleanPath[len(prefix)] == '/' || prefix[len(prefix)-1] == '/') {
					if len(prefix) > longestPrefix {
						pathMatched = true
						if isMethodAllowed(entry.config.Methods, req.Method) {
							longestPrefix = len(prefix)
							matchedHandler = entry.handler
							methodMatched = true
						}
					}
				}
			}
		}
	}

	if matchedHandler != nil {
		matchedHandler.ServeHTTP(w, req)
		return
	}

	if pathMatched && !methodMatched {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed",
			fmt.Sprintf("HTTP method %s is not allowed for this route", req.Method), cleanPath)
		return
	}

	writeJSONError(w, http.StatusNotFound, "Not Found", "No matching route found for the requested path", cleanPath)
}

func isMethodAllowed(allowedMethods []string, reqMethod string) bool {
	if len(allowedMethods) == 0 {
		return true
	}
	for _, m := range allowedMethods {
		if strings.EqualFold(m, reqMethod) {
			return true
		}
	}
	return false
}

func writeJSONError(w http.ResponseWriter, statusCode int, errType, message, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := ErrorResponse{
		Error:     errType,
		Message:   message,
		Path:      path,
		Code:      statusCode,
		Timestamp: time.Now().UTC(),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
