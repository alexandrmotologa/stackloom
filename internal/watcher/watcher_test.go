package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileWatcherDetectsChange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "docker-compose.yml")

	if err := os.WriteFile(testFile, []byte("version: '3'"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	var triggered int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := NewFileWatcher([]string{testFile}, 50*time.Millisecond, func() {
		atomic.AddInt32(&triggered, 1)
	})

	go w.Start(ctx)

	// Sleep slightly, then modify file
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("version: '3'\nservices:\n  web:\n    image: nginx"), 0644); err != nil {
		t.Fatalf("failed to update file: %v", err)
	}

	// Wait for watcher to trigger
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&triggered) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if atomic.LoadInt32(&triggered) == 0 {
		t.Errorf("expected file watcher to trigger change callback")
	}
}
