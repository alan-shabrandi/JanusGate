package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"janusgate/internal/config"
	"janusgate/internal/proxy"
)

var (
	ErrRouteNotFound = errors.New("route not found")
	ErrInvalidPath   = errors.New("invalid route path")
)

type Router interface {
	AddRoute(route config.RouteConfig, handler http.Handler) error
	LoadRoutes(routes []config.RouteConfig) error
	http.Handler
}

type routeEntry struct {
	config  config.RouteConfig
	handler http.Handler
}

type memoryRouter struct {
	mu     sync.RWMutex
	routes []routeEntry
}

func NewRouter() Router {
	return &memoryRouter{
		routes: make([]routeEntry, 0),
	}
}

func (r *memoryRouter) AddRoute(route config.RouteConfig, handler http.Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if route.Path == "" {
		return ErrInvalidPath
	}

	if route.MatchType == "" {
		route.MatchType = "prefix"
	}

	r.routes = append(r.routes, routeEntry{
		config:  route,
		handler: handler,
	})

	return nil
}

func (r *memoryRouter) LoadRoutes(routes []config.RouteConfig) error {
	for _, route := range routes {
		if len(route.Upstreams) == 0 {
			log.Printf("Warning: Route [%s] has no upstreams configured, skipping...", route.ID)
			continue
		}

		targetURL := route.Upstreams[0].URL
		revProxy, err := proxy.NewProxy(targetURL)
		if err != nil {
			return fmt.Errorf("failed to create proxy for route %s: %w", route.ID, err)
		}

		var handler http.Handler = revProxy

		if route.StripPrefix {
			handler = proxy.StripPrefix(route.Path, revProxy)
		}

		if err := r.AddRoute(route, handler); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.ID, err)
		}

		log.Printf("Loaded Route [%s]: Path=%s -> Target=%s (Type: %s, StripPrefix: %t)",
			route.ID, route.Path, targetURL, route.MatchType, route.StripPrefix)
	}
	return nil
}

func (r *memoryRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	path := req.URL.Path
	var matchedHandler http.Handler
	longestPrefix := -1

	for _, entry := range r.routes {
		if entry.config.MatchType == "exact" && entry.config.Path == path {
			matchedHandler = entry.handler
			break
		}
	}

	if matchedHandler == nil {
		for _, entry := range r.routes {
			if entry.config.MatchType == "prefix" || entry.config.MatchType == "" {
				prefix := entry.config.Path
				if strings.HasPrefix(path, prefix) && len(prefix) > longestPrefix {
					longestPrefix = len(prefix)
					matchedHandler = entry.handler
				}
			}
		}
	}

	if matchedHandler != nil {
		matchedHandler.ServeHTTP(w, req)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "Not Found",
		"message": "No matching route found for the requested path",
		"code":    404,
	})
}
