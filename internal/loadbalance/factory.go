package loadbalance

import "strings"

func NewBalancer(strategy string) Balancer {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "weighted_round_robin", "wrr":
		return NewWeightedRoundRobin()
	case "least_connections", "least_conn", "lc":
		return NewLeastConnections()
	case "ip_hash", "iphash":
		return NewIPHash()
	case "round_robin", "rr", "":
		fallthrough
	default:
		return NewRoundRobin()
	}
}
