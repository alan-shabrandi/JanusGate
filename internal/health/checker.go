package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"janusgate/internal/config"
	"janusgate/internal/upstream"
)

type Checker struct {
	registry   *upstream.Registry
	client     *http.Client
	interval   time.Duration
	healthPath string
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
		interval:   interval,
		healthPath: "/health",
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
	statuses := c.registry.GetAllStatuses()

	for url := range statuses {
		go c.checkUpstream(ctx, url)
	}
}

func (c *Checker) checkUpstream(ctx context.Context, targetURL string) {
	fullURL := targetURL + c.healthPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		c.registry.SetHealth(targetURL, false)
		return
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.registry.SetHealth(targetURL, false)
		return
	}
	defer resp.Body.Close()

	isHealthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	c.registry.SetHealth(targetURL, isHealthy)
}
