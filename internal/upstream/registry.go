package upstream

import (
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	URL       string
	Weight    int
	IsHealthy atomic.Bool
	LastCheck atomic.Int64
}
type ServerSnapshot struct {
	URL       string    `json:"url"`
	Weight    int       `json:"weight"`
	IsHealthy bool      `json:"is_healthy"`
	LastCheck time.Time `json:"last_check"`
}

type Registry struct {
	mu         sync.RWMutex
	servers    map[string]*Server
	routePools map[string][]*Server
}

func NewRegistry() *Registry {
	return &Registry{
		servers:    make(map[string]*Server),
		routePools: make(map[string][]*Server),
	}
}

func (r *Registry) RegisterServer(routeID, targetURL string, weight int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if weight <= 0 {
		weight = 1
	}

	srv, exists := r.servers[targetURL]
	if !exists {
		srv = &Server{
			URL:    targetURL,
			Weight: weight,
		}
		srv.IsHealthy.Store(true)
		srv.LastCheck.Store(time.Now().UnixNano())
		r.servers[targetURL] = srv
	}

	pool := r.routePools[routeID]
	for _, existing := range pool {
		if existing.URL == targetURL {
			return
		}
	}

	newPool := make([]*Server, len(pool), len(pool)+1)
	copy(newPool, pool)
	newPool = append(newPool, srv)
	r.routePools[routeID] = newPool
}

func (r *Registry) SetHealth(targetURL string, isHealthy bool) {
	r.mu.RLock()
	server, exists := r.servers[targetURL]
	r.mu.RUnlock()

	if exists {
		server.IsHealthy.Store(isHealthy)
		server.LastCheck.Store(time.Now().UnixNano())
	}
}

func (r *Registry) GetServers(routeID string) []*Server {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.routePools[routeID]
}

func (r *Registry) GetAllStatuses() map[string]ServerSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make(map[string]ServerSnapshot, len(r.servers))
	for url, server := range r.servers {
		snapshot[url] = ServerSnapshot{
			URL:       server.URL,
			Weight:    server.Weight,
			IsHealthy: server.IsHealthy.Load(),
			LastCheck: time.Unix(0, server.LastCheck.Load()),
		}
	}
	return snapshot
}
