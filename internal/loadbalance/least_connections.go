package loadbalance

import (
	"janusgate/internal/upstream"
)

type leastConnectionsBalancer struct{}

func NewLeastConnections() Balancer {
	return &leastConnectionsBalancer{}
}

func (lc *leastConnectionsBalancer) Next(servers []*upstream.Server) (*upstream.Server, error) {
	if len(servers) == 0 {
		return nil, ErrNoAvailableServer
	}

	var best *upstream.Server
	var minConns int64 = -1

	for _, srv := range servers {
		conns := srv.ActiveConns.Load()
		if minConns == -1 || conns < minConns {
			minConns = conns
			best = srv
		}
	}

	if best == nil {
		return nil, ErrNoAvailableServer
	}

	return best, nil
}
