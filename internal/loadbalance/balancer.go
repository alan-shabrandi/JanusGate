package loadbalance

import (
	"errors"

	"janusgate/internal/upstream"
)

var ErrNoAvailableServer = errors.New("no available healthy upstream server")

type Balancer interface {
	Next(servers []*upstream.Server) (*upstream.Server, error)
}

type KeyedBalancer interface {
	Balancer
	NextWithKey(servers []*upstream.Server, key string) (*upstream.Server, error)
}
