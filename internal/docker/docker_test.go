package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDockerClientOperationsWithMockServer(t *testing.T) {
	// Create mock Docker Engine server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/_ping":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))

		case r.URL.Path == "/v1.41/containers/json":
			containers := []rawContainerJSON{
				{
					ID:      "container-12345",
					Names:   []string{"/my-app_postgres_1"},
					Image:   "postgres:16",
					State:   "running",
					Status:  "Up 3 hours (healthy)",
					Created: time.Now().Unix(),
					Labels: map[string]string{
						"com.docker.compose.project": "my-app",
						"com.docker.compose.service": "postgres",
					},
					Ports: []struct {
						IP          string `json:"IP"`
						PrivatePort int    `json:"PrivatePort"`
						PublicPort  int    `json:"PublicPort"`
						Type        string `json:"Type"`
					}{
						{IP: "0.0.0.0", PrivatePort: 5432, PublicPort: 5432, Type: "tcp"},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(containers)

		case r.URL.Path == "/v1.41/containers/container-12345/stats":
			stats := rawDockerStats{}
			stats.CPUStats.CPUUsage.TotalUsage = 500000000
			stats.PreCPUStats.CPUUsage.TotalUsage = 400000000
			stats.CPUStats.SystemCPUUsage = 2000000000
			stats.PreCPUStats.SystemCPUUsage = 1000000000
			stats.CPUStats.OnlineCPUs = 2
			stats.MemoryStats.Usage = 150 * 1024 * 1024
			stats.MemoryStats.Limit = 1024 * 1024 * 1024
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(stats)

		case r.URL.Path == "/v1.41/containers/container-12345/logs":
			// Write multiplexed frames: frame 1 stdout, frame 2 stderr
			buf := new(bytes.Buffer)

			// Stdout frame
			msg1 := "2026-11-10T15:00:00Z database system is ready to accept connections\n"
			h1 := make([]byte, 8)
			h1[0] = 1 // stdout
			binary.BigEndian.PutUint32(h1[4:8], uint32(len(msg1)))
			buf.Write(h1)
			buf.WriteString(msg1)

			// Stderr frame
			msg2 := "2026-11-10T15:00:01Z warning: check checkpoint frequency\n"
			h2 := make([]byte, 8)
			h2[0] = 2 // stderr
			binary.BigEndian.PutUint32(h2[4:8], uint32(len(msg2)))
			buf.Write(h2)
			buf.WriteString(msg2)

			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			_, _ = w.Write(buf.Bytes())

		case r.URL.Path == "/v1.41/containers/container-12345/start":
			w.WriteHeader(http.StatusNoContent)

		case r.URL.Path == "/v1.41/containers/container-12345/stop":
			w.WriteHeader(http.StatusNoContent)

		case r.URL.Path == "/v1.41/containers/container-12345/restart":
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Initialize client with mock server URL (tcp/http)
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("failed to initialize client: %v", err)
	}

	ctx := context.Background()

	// 1. Ping
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	if !client.IsConnected() {
		t.Errorf("expected client to be connected")
	}

	// 2. List containers
	containers, err := client.ListContainers(ctx, "my-app")
	if err != nil {
		t.Fatalf("list containers failed: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].Service != "postgres" || containers[0].Health != "healthy" {
		t.Errorf("unexpected container data: %+v", containers[0])
	}

	// 3. Stats
	metrics, err := client.GetStats(ctx, "container-12345")
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if metrics.CPUPercentage <= 0 {
		t.Errorf("expected positive CPU percentage, got %f", metrics.CPUPercentage)
	}
	if metrics.MemoryUsage == 0 {
		t.Errorf("expected non-zero memory usage")
	}

	// 4. Logs streaming
	linesCh, errCh := client.StreamLogs(ctx, "container-12345", 10, false)
	var received []LogLine
	for l := range linesCh {
		received = append(received, l)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("logs error: %v", err)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(received))
	}
	if received[0].Stream != StreamStdout || received[1].Stream != StreamStderr {
		t.Errorf("unexpected stream types in logs: %+v", received)
	}

	// 5. Container actions
	if err := client.StartContainer(ctx, "container-12345"); err != nil {
		t.Errorf("start failed: %v", err)
	}
	if err := client.StopContainer(ctx, "container-12345"); err != nil {
		t.Errorf("stop failed: %v", err)
	}
	if err := client.RestartContainer(ctx, "container-12345"); err != nil {
		t.Errorf("restart failed: %v", err)
	}
}

func TestDockerClientOfflineFallback(t *testing.T) {
	// Point to non-existent endpoint
	client, err := NewClient("tcp://127.0.0.1:54399")
	if err != nil {
		t.Fatalf("NewClient should not fail initialization: %v", err)
	}

	if client.IsConnected() {
		t.Errorf("expected client to report not connected")
	}

	if client.LastError() == "" {
		t.Errorf("expected LastError to describe connection failure")
	}
}
