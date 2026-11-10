package graph

import (
	"testing"

	"github.com/alexandrmotologa/stackloom/internal/compose"
)

func TestComputeNetworkTopology(t *testing.T) {
	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"frontend": {
				Networks: []string{"public-net"},
			},
			"backend": {
				Networks: []string{"public-net", "internal-net"},
			},
			"db": {
				Networks: []string{"internal-net"},
			},
			"standalone-island": {
				Networks: []string{"island-net"},
			},
		},
	}

	topo := ComputeNetworkTopology(cfg)

	if len(topo.Networks) != 3 {
		t.Fatalf("expected 3 networks, got %d", len(topo.Networks))
	}

	// Backend can reach frontend and db
	backendReachable := topo.VisibilityMatrix["backend"]
	if len(backendReachable) != 2 {
		t.Errorf("expected backend to reach 2 services, got %v", backendReachable)
	}

	// Frontend CANNOT reach db
	frontendReachable := topo.VisibilityMatrix["frontend"]
	if len(frontendReachable) != 1 || frontendReachable[0] != "backend" {
		t.Errorf("frontend should only reach backend, got %v", frontendReachable)
	}

	// standalone-island is isolated from all other services
	if len(topo.IsolatedServices) != 1 || topo.IsolatedServices[0] != "standalone-island" {
		t.Errorf("expected standalone-island to be isolated, got %v", topo.IsolatedServices)
	}
}
