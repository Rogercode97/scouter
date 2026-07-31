package engine

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher provides a debounced file system watcher that triggers re-indexing.
type Watcher struct {
	logger *slog.Logger
	mu     sync.Mutex
	active map[string]bool // keep track of paths we are currently watching (rootUri)
}

func NewWatcher(logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Watcher{
		logger: logger,
		active: make(map[string]bool),
	}
}

// Start watching a directory for changes. Returns immediately.
func (w *Watcher) Start(ctx context.Context, pathPrefix string, indexFunc func(ctx context.Context, dir string) error) error {
	w.mu.Lock()
	if w.active[pathPrefix] {
		w.mu.Unlock()
		return nil // Already watching this path
	}
	w.active[pathPrefix] = true
	w.mu.Unlock()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	w.logger.Info("Starting file watcher for auto-indexing", "path", pathPrefix)

	// Recursively add directories to the watcher
	err = filepath.WalkDir(pathPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip toxic/heavy directories
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "build" || base == ".venv" || base == ".tox" || base == "target" || base == "bin" || base == ".next" || base == ".gemini" {
				return filepath.SkipDir
			}
			watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		watcher.Close()
		return err
	}

	go func() {
		defer watcher.Close()
		defer func() {
			w.mu.Lock()
			delete(w.active, pathPrefix)
			w.mu.Unlock()
		}()

		var (
			timer *time.Timer
			mu    sync.Mutex
			delay = 250 * time.Millisecond
		)

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Only react to Write, Create, or Remove
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					mu.Lock()
					if timer != nil {
						timer.Stop()
					}
					// Check if a new directory was created, to recursively add it to the watcher
					if event.Has(fsnotify.Create) {
						if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
							filepath.WalkDir(event.Name, func(path string, d fs.DirEntry, err error) error {
								if err != nil {
									return nil
								}
								if d.IsDir() {
									base := d.Name()
									if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "build" || base == ".venv" || base == ".tox" || base == "target" || base == "bin" || base == ".next" || base == ".gemini" {
										return filepath.SkipDir
									}
									watcher.Add(path)
								}
								return nil
							})
						}
					}

					timer = time.AfterFunc(delay, func() {
						w.logger.Info("File changes detected. Debounce finished. Triggering indexer...", "event", event.Name)
						// Run index tied to the server's lifecycle context
						if err := indexFunc(ctx, pathPrefix); err != nil {
							w.logger.Error("Auto-indexer failed", "error", err)
						}
					})
					mu.Unlock()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				w.logger.Warn("Watcher error", "error", err)
			}
		}
	}()

	return nil
}
