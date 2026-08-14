package upstream

import (
	"sync"
	"time"
)

type Server struct {
	URL       string    `json:"url"`
	Weight    int       `json:"weight"`
	IsHealthy bool      `json:"is_healthy"`
	LastCheck time.Time `json:"last_check"`
}

type Registry struct {
	mu         sync.RWMutex
	servers    map[string]*Server
	routePools map[string][]string
}

func NewRegistry() *Registry {
	return &Registry{
		servers:    make(map[string]*Server),
		routePools: make(map[string][]string),
	}
}

func (r *Registry) RegisterServer(routeID, targetURL string, weight int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if weight <= 0 {
		weight = 1
	}

	if _, exists := r.servers[targetURL]; !exists {
		r.servers[targetURL] = &Server{
			URL:       targetURL,
			Weight:    weight,
			IsHealthy: true,
			LastCheck: time.Now(),
		}
	}

	r.routePools[routeID] = append(r.routePools[routeID], targetURL)
}

func (r *Registry) SetHealth(targetURL string, isHealthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if server, exists := r.servers[targetURL]; exists {
		server.IsHealthy = isHealthy
		server.LastCheck = time.Now()
	}
}

func (r *Registry) IsHealthy(targetURL string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if server, exists := r.servers[targetURL]; exists {
		return server.IsHealthy
	}
	return false
}

func (r *Registry) GetHealthyServers(routeID string) []*Server {
	r.mu.RLock()
	defer r.mu.RUnlock()

	urls, exists := r.routePools[routeID]
	if !exists {
		return nil
	}

	healthy := make([]*Server, 0, len(urls))
	for _, url := range urls {
		if server, ok := r.servers[url]; ok && server.IsHealthy {
			srvCopy := *server
			healthy = append(healthy, &srvCopy)
		}
	}

	return healthy
}

func (r *Registry) GetAllStatuses() map[string]Server {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make(map[string]Server, len(r.servers))
	for url, server := range r.servers {
		snapshot[url] = *server
	}
	return snapshot
}
