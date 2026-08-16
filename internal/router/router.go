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
	"janusgate/internal/loadbalance"
	"janusgate/internal/middleware"
	"janusgate/internal/proxy"
	"janusgate/internal/upstream"
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
	registry *upstream.Registry
}

func NewRouter(routes []config.RouteConfig, tokenMgr auth.TokenManager, reg *upstream.Registry) Router {
	r := &memoryRouter{
		tokenMgr: tokenMgr,
		registry: reg,
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

		if r.registry != nil {
			for _, u := range route.Upstreams {
				r.registry.RegisterServer(route.ID, u.URL, u.Weight)
			}
		}

		cbConfig := circuitbreaker.Config{
			Name:               route.ID,
			MaxRequests:        1,
			Timeout:            10 * time.Second,
			MinRequestsToTrip:  5,
			FailureRatioToTrip: 0.5,
		}

		upstreamProxies := make(map[string]http.Handler, len(route.Upstreams))

		for _, u := range route.Upstreams {
			revProxy, err := proxy.NewProxy(u.URL, &cbConfig, route.Retry, r.registry)
			if err != nil {
				return fmt.Errorf("failed to create proxy for route %s (upstream %s): %w", route.ID, u.URL, err)
			}
			upstreamProxies[u.URL] = revProxy
		}

		balancer := loadbalance.NewBalancer(route.LBStrategy)
		routeID := route.ID

		var dynamicHandler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if r.registry == nil {
				if primary, ok := upstreamProxies[route.Upstreams[0].URL]; ok {
					primary.ServeHTTP(w, req)
					return
				}
				writeJSONError(w, http.StatusServiceUnavailable, "Service Unavailable", "No upstream available", req.URL.Path)
				return
			}

			healthyServers := r.registry.GetHealthyServers(routeID)
			if len(healthyServers) == 0 {
				writeJSONError(w, http.StatusServiceUnavailable, "Service Unavailable", "No healthy upstream servers available for this route", req.URL.Path)
				return
			}

			var selectedServer *upstream.Server
			var err error

			if keyedBalancer, ok := balancer.(loadbalance.KeyedBalancer); ok {
				clientIP := getClientIP(req)
				selectedServer, err = keyedBalancer.NextWithKey(healthyServers, clientIP)
			} else {
				selectedServer, err = balancer.Next(healthyServers)
			}

			if err != nil || selectedServer == nil {
				writeJSONError(w, http.StatusServiceUnavailable, "Service Unavailable", "Failed to select healthy upstream server", req.URL.Path)
				return
			}

			handler, ok := upstreamProxies[selectedServer.URL]
			if !ok {
				writeJSONError(w, http.StatusBadGateway, "Bad Gateway", "Selected upstream handler not found", req.URL.Path)
				return
			}

			selectedServer.ActiveConns.Add(1)
			defer selectedServer.ActiveConns.Add(-1)

			handler.ServeHTTP(w, req)
		})

		if route.StripPrefix {
			dynamicHandler = proxy.StripPrefix(route.PathPrefix, dynamicHandler)
		}

		pipeline := middleware.New()
		pipeline = pipeline.Use(middleware.Timeout(route.Timeout))

		if route.RequiresAuth {
			if r.tokenMgr == nil {
				slog.Error("Route requires authentication but TokenManager is nil", "route_id", route.ID)
			} else {
				pipeline = pipeline.Use(middleware.Authenticate(r.tokenMgr))
				slog.Info("Route loaded [PRIVATE]", "id", route.ID, "path", route.PathPrefix, "upstreams_count", len(route.Upstreams), "lb_strategy", route.LBStrategy)
			}
		} else {
			slog.Info("Route loaded [PUBLIC]", "id", route.ID, "path", route.PathPrefix, "upstreams_count", len(route.Upstreams), "lb_strategy", route.LBStrategy)
		}

		finalHandler := pipeline.Then(dynamicHandler)

		newRoutes = append(newRoutes, routeEntry{
			config:  route,
			handler: finalHandler,
		})
	}

	r.routes.Store(&newRoutes)
	slog.Info("Router state atomically updated with new routing table and load balancers", "total_routes", len(newRoutes))

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

func getClientIP(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xreal := req.Header.Get("X-Real-IP"); xreal != "" {
		return strings.TrimSpace(xreal)
	}
	return req.RemoteAddr
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
