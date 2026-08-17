package loadbalance

import (
	"sync/atomic"

	"janusgate/internal/upstream"
)

type leastConnectionsBalancer struct {
	counter uint64
}

func NewLeastConnections() Balancer {
	return &leastConnectionsBalancer{}
}

func (lc *leastConnectionsBalancer) Next(servers []*upstream.Server) (*upstream.Server, error) {
	if len(servers) == 0 {
		return nil, ErrNoAvailableServer
	}

	var minConns int64 = -1
	var best []*upstream.Server

	for _, srv := range servers {
		conns := srv.ActiveConns.Load()
		if minConns == -1 || conns < minConns {
			minConns = conns
			best = []*upstream.Server{srv}
		} else if conns == minConns {
			best = append(best, srv)
		}
	}

	if len(best) == 0 {
		return nil, ErrNoAvailableServer
	}

	if len(best) == 1 {
		return best[0], nil
	}

	n := uint64(len(best))
	idx := atomic.AddUint64(&lc.counter, 1) - 1
	return best[idx%n], nil
}
