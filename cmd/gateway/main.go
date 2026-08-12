package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"janusgate/internal/config"
	"janusgate/internal/proxy"
	"janusgate/internal/server"
)

func main() {
	// ۱. بارگذاری کانفیگ
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mux := http.NewServeMux()

	// ۲. اتصال پویا مسیرها به پروکسی‌ها
	for _, route := range cfg.Routes {
		if len(route.Upstreams) == 0 {
			log.Printf("Warning: Route %s has no upstreams configured, skipping...", route.ID)
			continue
		}

		// استفاده از اولین upstream (منطق لودبالانسینگ در روزهای آینده اضافه خواهد شد)
		targetURL := route.Upstreams[0].URL
		revProxy, err := proxy.NewProxy(targetURL)
		if err != nil {
			log.Fatalf("Failed to create proxy for route %s: %v", route.ID, err)
		}

		var handler http.Handler = revProxy

		// اعمال StripPrefix در صورت فعال بودن در کانفیگ
		if route.StripPrefix {
			handler = http.StripPrefix(route.Path, revProxy)
		}

		pattern := route.Path
		if !strings.HasSuffix(pattern, "/") {
			pattern += "/"
		}

		mux.Handle(pattern, handler)
		log.Printf("Mapped Route [%s]: %s -> %s (StripPrefix: %t)", route.ID, route.Path, targetURL, route.StripPrefix)
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("JanusGate is Healthy"))
	})

	srv := server.NewServer(&cfg.Server, mux)
	srv.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	_ = srv.Shutdown(10 * time.Second)
}
