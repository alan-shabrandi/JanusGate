package health

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"janusgate/internal/config"
)

type UpstreamStatus struct {
	URL       string    `json:"url"`
	IsHealthy bool      `json:"is_healthy"`
	LastCheck time.Time `json:"last_check"`
}

type Checker struct {
	mu         sync.RWMutex
	statuses   map[string]*UpstreamStatus
	client     *http.Client
	interval   time.Duration
	healthPath string
}

func NewChecker(interval time.Duration) *Checker {
	if interval <= 0 {
		interval = 10 * time.Second
	}

	return &Checker{
		statuses: make(map[string]*UpstreamStatus),
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		interval:   interval,
		healthPath: "/health",
	}
}

func (c *Checker) RegisterUpstream(targetURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.statuses[targetURL]; !exists {
		c.statuses[targetURL] = &UpstreamStatus{
			URL:       targetURL,
			IsHealthy: true,
			LastCheck: time.Now(),
		}
	}
}

func (c *Checker) RegisterRoutesUpstreams(routes []config.RouteConfig) {
	for _, route := range routes {
		for _, upstream := range route.Upstreams {
			c.RegisterUpstream(upstream.URL)
		}
	}
}

func (c *Checker) Start(ctx context.Context) {
	slog.Info("Starting Active Health Checker...", "interval", c.interval.String())

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
	c.mu.RLock()
	urls := make([]string, 0, len(c.statuses))
	for url := range c.statuses {
		urls = append(urls, url)
	}
	c.mu.RUnlock()

	var wg sync.WaitGroup
	for _, targetURL := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			c.checkUpstream(ctx, u)
		}(targetURL)
	}
	wg.Wait()
}

func (c *Checker) checkUpstream(ctx context.Context, targetURL string) {
	fullURL := targetURL + c.healthPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		c.markStatus(targetURL, false)
		return
	}

	resp, err := c.client.Do(req)
	if err != nil {
		slog.Warn("Upstream health check failed (Network/Timeout)", "url", targetURL, "error", err)
		c.markStatus(targetURL, false)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.markStatus(targetURL, true)
	} else {
		slog.Warn("Upstream health check failed (HTTP Status)", "url", targetURL, "status", resp.StatusCode)
		c.markStatus(targetURL, false)
	}
}

func (c *Checker) markStatus(targetURL string, isHealthy bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	status, exists := c.statuses[targetURL]
	if !exists {
		return
	}

	if status.IsHealthy != isHealthy {
		slog.Warn("Upstream health status changed",
			"url", targetURL,
			"healthy", isHealthy,
		)
	}

	status.IsHealthy = isHealthy
	status.LastCheck = time.Now()
}

func (c *Checker) IsHealthy(targetURL string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if status, exists := c.statuses[targetURL]; exists {
		return status.IsHealthy
	}
	return false
}

func (c *Checker) GetStatuses() map[string]UpstreamStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]UpstreamStatus, len(c.statuses))
	for k, v := range c.statuses {
		result[k] = *v
	}
	return result
}
