package health

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"janusgate/internal/config"
	"janusgate/internal/upstream"
)

type targetState struct {
	consecutiveSuccesses int
	consecutiveFailures  int
	isDown               bool
}

type Checker struct {
	registry      *upstream.Registry
	client        *http.Client
	interval      time.Duration
	healthPath    string
	fallThreshold int
	riseThreshold int

	mu     sync.Mutex
	states map[string]*targetState
}

func NewChecker(registry *upstream.Registry, interval time.Duration) *Checker {
	if interval <= 0 {
		interval = 10 * time.Second
	}

	return &Checker{
		registry: registry,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		interval:      interval,
		healthPath:    "/health",
		fallThreshold: 3,
		riseThreshold: 2,
		states:        make(map[string]*targetState),
	}
}

func (c *Checker) RegisterRoutesUpstreams(routes []config.RouteConfig) {
	for _, route := range routes {
		for _, up := range route.Upstreams {
			c.registry.RegisterServer(route.ID, up.URL, up.Weight)
		}
	}
}

func (c *Checker) Start(ctx context.Context) {
	slog.Info("Starting Active Health Checker", "interval", c.interval.String())

	c.checkAll(ctx)

	ticker := time.NewTicker(c.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("Stopping Active Health Checker...")
				return
			case <-ticker.C:
				c.checkAll(ctx)
			}
		}
	}()
}

func (c *Checker) checkAll(ctx context.Context) {
	statuses := c.registry.GetAllStatuses()

	for url := range statuses {
		go c.checkUpstream(ctx, url)
	}
}

func (c *Checker) checkUpstream(ctx context.Context, targetURL string) {
	fullURL := strings.TrimRight(targetURL, "/") + c.healthPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		c.updateState(targetURL, false, err.Error())
		return
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.updateState(targetURL, false, err.Error())
		return
	}
	defer resp.Body.Close()

	isHealthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	c.updateState(targetURL, isHealthy, resp.Status)
}

func (c *Checker) updateState(targetURL string, isHealthy bool, reason string) {
	c.mu.Lock()
	state, exists := c.states[targetURL]
	if !exists {
		state = &targetState{}
		c.states[targetURL] = state
	}

	stateChanged := false
	markAsDown := state.isDown

	if isHealthy {
		state.consecutiveFailures = 0
		state.consecutiveSuccesses++

		if state.isDown && state.consecutiveSuccesses >= c.riseThreshold {
			state.isDown = false
			stateChanged = true
			markAsDown = false
		}
	} else {
		state.consecutiveSuccesses = 0
		state.consecutiveFailures++

		if !state.isDown && state.consecutiveFailures >= c.fallThreshold {
			state.isDown = true
			stateChanged = true
			markAsDown = true
		}
	}
	c.mu.Unlock()

	if stateChanged {
		if markAsDown {
			slog.Warn("Upstream marked DOWN (Health Check Failed)",
				"target", targetURL,
				"reason", reason,
				"failures", c.fallThreshold)
			c.registry.SetHealth(targetURL, false)
		} else {
			slog.Info("Upstream marked UP (Health Check Recovered)",
				"target", targetURL,
				"successes", c.riseThreshold)
			c.registry.SetHealth(targetURL, true)
		}
	}
}
