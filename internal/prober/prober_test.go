package prober

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alexandrmotologa/stackloom/internal/compose"
)

func TestProbePublishedEndpoints(t *testing.T) {
	// Start a mock live HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Extract port from server.URL
	portStr := server.URL[strings.LastIndex(server.URL, ":")+1:]
	port, _ := strconv.Atoi(portStr)

	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"live-web": {
				Ports: []compose.PortMapping{
					{Target: 80, Published: port, Protocol: "tcp"},
				},
			},
			"dead-service": {
				Ports: []compose.PortMapping{
					{Target: 9999, Published: 59999, Protocol: "tcp"},
				},
			},
		},
	}

	ctx := context.Background()
	results := ProbePublishedEndpoints(ctx, cfg)

	if len(results) != 2 {
		t.Fatalf("expected 2 probe results, got %d", len(results))
	}

	var liveFound, deadFound bool
	for _, p := range results {
		if p.Service == "live-web" {
			liveFound = true
			if !p.Live || p.StatusCode != 200 {
				t.Errorf("expected live-web to be live 200, got: %+v", p)
			}
		}
		if p.Service == "dead-service" {
			deadFound = true
			if p.Live {
				t.Errorf("expected dead-service to not be live, got: %+v", p)
			}
		}
	}

	if !liveFound || !deadFound {
		t.Errorf("expected both services in probe results")
	}
}
