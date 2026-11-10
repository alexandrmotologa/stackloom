package graph

import (
	"testing"
)

func TestCalculateBlastRadius(t *testing.T) {
	cfg := createTestConfig() // db, redis, backend, worker, frontend
	g := BuildGraph(cfg)

	// Outage on db:
	// - backend depends directly on db (condition: healthy -> FATAL)
	// - frontend depends on backend -> depth 2
	// - worker and redis should be UNAFFECTED
	report := g.CalculateBlastRadius("db")

	if report.TargetService != "db" {
		t.Errorf("expected target db, got %s", report.TargetService)
	}
	if report.TotalServices != 5 {
		t.Errorf("expected 5 total services, got %d", report.TotalServices)
	}
	if report.AffectedCount != 2 {
		t.Errorf("expected 2 affected services (backend, frontend), got %d", report.AffectedCount)
	}

	foundBackend := false
	foundFrontend := false
	for _, n := range report.ImpactTree {
		if n.Service == "backend" {
			foundBackend = true
			if n.Depth != 1 || n.Severity != SeverityFatal {
				t.Errorf("backend impact invalid: %+v", n)
			}
		}
		if n.Service == "frontend" {
			foundFrontend = true
			if n.Depth != 2 {
				t.Errorf("frontend impact depth expected 2, got %d", n.Depth)
			}
		}
	}

	if !foundBackend || !foundFrontend {
		t.Errorf("expected backend and frontend in impact tree")
	}

	if len(report.Unaffected) != 2 {
		t.Errorf("expected 2 unaffected services (redis, worker), got %d: %v", len(report.Unaffected), report.Unaffected)
	}
}
