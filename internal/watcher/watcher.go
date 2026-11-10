package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileWatcher polls watched files and directories for modifications.
type FileWatcher struct {
	paths    []string
	interval time.Duration
	debounce time.Duration
	mu       sync.Mutex
	modTimes map[string]time.Time
	fileSizes map[string]int64
	onChange func()
}

// NewFileWatcher initializes a file watcher with specified polling interval.
func NewFileWatcher(paths []string, interval time.Duration, onChange func()) *FileWatcher {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	w := &FileWatcher{
		paths:     paths,
		interval:  interval,
		debounce:  200 * time.Millisecond,
		modTimes:  make(map[string]time.Time),
		fileSizes: make(map[string]int64),
		onChange:  onChange,
	}
	w.recordInitialState()
	return w
}

func (w *FileWatcher) recordInitialState() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, p := range w.paths {
		if info, err := os.Stat(p); err == nil {
			w.modTimes[p] = info.ModTime()
			w.fileSizes[p] = info.Size()
		}
	}
}

// AddPath adds a new file or directory to watch.
func (w *FileWatcher) AddPath(p string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	for _, existing := range w.paths {
		if existing == p {
			return
		}
	}
	w.paths = append(w.paths, p)
	if info, err := os.Stat(p); err == nil {
		w.modTimes[p] = info.ModTime()
		w.fileSizes[p] = info.Size()
	}
}

// Start runs the polling loop until the context is canceled.
func (w *FileWatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	var lastTrigger time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed := false
			w.mu.Lock()
			for _, p := range w.paths {
				info, err := os.Stat(p)
				if err != nil {
					// File might have been deleted
					if _, existed := w.modTimes[p]; existed {
						delete(w.modTimes, p)
						delete(w.fileSizes, p)
						changed = true
					}
					continue
				}

				oldTime, hasTime := w.modTimes[p]
				oldSize, hasSize := w.fileSizes[p]

				if !hasTime || !hasSize || !info.ModTime().Equal(oldTime) || info.Size() != oldSize {
					w.modTimes[p] = info.ModTime()
					w.fileSizes[p] = info.Size()
					changed = true
				}
			}
			w.mu.Unlock()

			if changed {
				now := time.Now()
				if now.Sub(lastTrigger) >= w.debounce {
					lastTrigger = now
					if w.onChange != nil {
						w.onChange()
					}
				}
			}
		}
	}
}
