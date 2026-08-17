package loadbalance

import (
	"sync"

	"janusgate/internal/upstream"
)

type weightedRoundRobinBalancer struct {
	mu             sync.Mutex
	currentWeights map[string]int
}

func NewWeightedRoundRobin() Balancer {
	return &weightedRoundRobinBalancer{
		currentWeights: make(map[string]int),
	}
}

func (w *weightedRoundRobinBalancer) Next(servers []*upstream.Server) (*upstream.Server, error) {
	if len(servers) == 0 {
		return nil, ErrNoAvailableServer
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.currentWeights) > len(servers) {
		activeURLs := make(map[string]struct{}, len(servers))
		for _, srv := range servers {
			activeURLs[srv.URL] = struct{}{}
		}
		for url := range w.currentWeights {
			if _, exists := activeURLs[url]; !exists {
				delete(w.currentWeights, url)
			}
		}
	}

	var best *upstream.Server
	totalWeight := 0

	for _, srv := range servers {
		weight := srv.Weight
		if weight <= 0 {
			weight = 1
		}

		totalWeight += weight
		w.currentWeights[srv.URL] += weight

		if best == nil || w.currentWeights[srv.URL] > w.currentWeights[best.URL] {
			best = srv
		}
	}

	if best != nil {
		w.currentWeights[best.URL] -= totalWeight
	}

	return best, nil
}
