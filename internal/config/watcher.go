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

type FileWatcher struct {
	watcher  *fsnotify.Watcher
	filePath string
	mu       sync.Mutex
	stopOnce sync.Once
	done     chan struct{}
}

func NewFileWatcher(filePath string) (*FileWatcher, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	dir := filepath.Dir(absPath)
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("failed to watch directory %s: %w", dir, err)
	}

	return &FileWatcher{
		watcher:  watcher,
		filePath: absPath,
		done:     make(chan struct{}),
	}, nil
}

func (fw *FileWatcher) Start(ctx context.Context, onChange func()) {
	var (
		debounceTimer *time.Timer
		debounceMu    sync.Mutex
	)

	const debounceDuration = 100 * time.Millisecond

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-fw.done:
				return
			case event, ok := <-fw.watcher.Events:
				if !ok {
					return
				}

				if filepath.Clean(event.Name) != fw.filePath {
					continue
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					slog.Debug("OS file event detected", "event", event.Op.String(), "file", event.Name)

					debounceMu.Lock()
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(debounceDuration, func() {
						slog.Info("Config file modification settled, triggering reload", "file", event.Name)
						onChange()
					})
					debounceMu.Unlock()
				}

			case err, ok := <-fw.watcher.Errors:
				if !ok {
					return
				}
				slog.Error("fsnotify watcher encountered error", "error", err)
			}
		}
	}()
}

func (fw *FileWatcher) Close() error {
	fw.stopOnce.Do(func() {
		close(fw.done)
	})
	return fw.watcher.Close()
}
