package loadbalance

import (
	"sync/atomic"

	"janusgate/internal/upstream"
)

type roundRobinBalancer struct {
	counter uint64
}

func NewRoundRobin() Balancer {
	return &roundRobinBalancer{}
}

func (rr *roundRobinBalancer) Next(servers []*upstream.Server) (*upstream.Server, error) {
	if len(servers) == 0 {
		return nil, ErrNoAvailableServer
	}

	n := uint64(len(servers))
	idx := atomic.AddUint64(&rr.counter, 1) - 1
	return servers[idx%n], nil
}
