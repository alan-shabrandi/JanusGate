package loadbalance

import (
	"hash/fnv"
	"net"
	"strings"
	"sync/atomic"

	"janusgate/internal/upstream"
)

type ipHashBalancer struct {
	fallbackCounter uint64
}

func NewIPHash() KeyedBalancer {
	return &ipHashBalancer{}
}

func (b *ipHashBalancer) Next(servers []*upstream.Server) (*upstream.Server, error) {
	if len(servers) == 0 {
		return nil, ErrNoAvailableServer
	}

	n := uint64(len(servers))
	idx := atomic.AddUint64(&b.fallbackCounter, 1) - 1
	return servers[idx%n], nil
}

func (b *ipHashBalancer) NextWithKey(servers []*upstream.Server, clientIP string) (*upstream.Server, error) {
	if len(servers) == 0 {
		return nil, ErrNoAvailableServer
	}

	cleanIP := sanitizeIP(clientIP)
	if cleanIP == "" {
		return b.Next(servers)
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(cleanIP))
	hashValue := h.Sum32()

	idx := int(hashValue % uint32(len(servers)))
	return servers[idx], nil
}

func sanitizeIP(clientIP string) string {
	ipStr := strings.TrimSpace(clientIP)
	if host, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = host
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	return ip.String()
}
