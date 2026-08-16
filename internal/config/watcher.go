package config

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type OnConfigChangeFunc func(newCfg *Config)

type BackgroundWatcher struct {
	configPath string
	manager    *Manager
	watcher    *fsnotify.Watcher
	mu         sync.Mutex
	stopChan   chan struct{}
}

func NewBackgroundWatcher(configPath string, manager *Manager) (*BackgroundWatcher, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute config path: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize fsnotify watcher: %w", err)
	}

	return &BackgroundWatcher{
		configPath: absPath,
		manager:    manager,
		watcher:    watcher,
		stopChan:   make(chan struct{}),
	}, nil
}

func (bw *BackgroundWatcher) Start(ctx context.Context, onChange OnConfigChangeFunc) error {
	dir := filepath.Dir(bw.configPath)
	if err := bw.watcher.Add(dir); err != nil {
		_ = bw.watcher.Close()
		return fmt.Errorf("failed to watch directory %s: %w", dir, err)
	}

	go bw.watchLoop(ctx, onChange)
	slog.Info("Background config watcher goroutine started", "target_file", bw.configPath)
	return nil
}

func (bw *BackgroundWatcher) watchLoop(ctx context.Context, onChange OnConfigChangeFunc) {
	var (
		debounceTimer *time.Timer
		timerMu       sync.Mutex
	)
	const debounceInterval = 150 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping config watcher background goroutine (context canceled)")
			return
		case <-bw.stopChan:
			slog.Info("Stopping config watcher background goroutine")
			return
		case event, ok := <-bw.watcher.Events:
			if !ok {
				return
			}

			if filepath.Clean(event.Name) != bw.configPath {
				continue
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				timerMu.Lock()
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceInterval, func() {
					slog.Info("Change detected in config file, re-parsing...", "file", event.Name)

					newCfg, err := bw.manager.Reload(bw.configPath)
					if err != nil {
						slog.Error("Failed to re-parse updated config file (retaining previous valid config)", "error", err)
						return
					}

					if onChange != nil {
						onChange(newCfg)
					}
				})
				timerMu.Unlock()
			}

		case err, ok := <-bw.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("Config watcher event loop error", "error", err)
		}
	}
}

func (bw *BackgroundWatcher) Stop() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	select {
	case <-bw.stopChan:
		return nil
	default:
		close(bw.stopChan)
	}
	return bw.watcher.Close()
}
