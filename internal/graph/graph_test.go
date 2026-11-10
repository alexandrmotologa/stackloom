package graph

import (
	"strings"
	"testing"

	"github.com/alexandrmotologa/stackloom/internal/compose"
)

func createTestConfig() *compose.ComposeConfig {
	return &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"db": {
				Image: "postgres:16",
				HealthCheck: &compose.HealthCheckConfig{
					Test: compose.CommandList{"pg_isready"},
				},
			},
			"redis": {
				Image: "redis:7",
			},
			"backend": {
				Image: "api:v1",
				DependsOn: map[string]compose.DependsOnCondition{
					"db":    {Condition: "service_healthy"},
					"redis": {Condition: "service_started"},
				},
			},
			"worker": {
				Image: "worker:v1",
				DependsOn: map[string]compose.DependsOnCondition{
					"redis": {Condition: "service_started"},
				},
			},
			"frontend": {
				Image: "ui:v1",
				DependsOn: map[string]compose.DependsOnCondition{
					"backend": {Condition: "service_started"},
				},
			},
		},
	}
}

func TestBuildGraphAndOrder(t *testing.T) {
	cfg := createTestConfig()
	g := BuildGraph(cfg)

	if len(g.Nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 4 {
		t.Fatalf("expected 4 edges, got %d", len(g.Edges))
	}

	// Verify cycle detection
	cycles := FindCycles(g)
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles in valid DAG, got %d: %+v", len(cycles), cycles)
	}

	// Verify startup plan
	plan, err := ComputeStartupPlan(g)
	if err != nil {
		t.Fatalf("failed to compute startup plan: %v", err)
	}

	if len(plan.Stages) != 3 {
		t.Fatalf("expected 3 startup stages, got %d", len(plan.Stages))
	}

	// Stage 0 should be db and redis
	stage0 := plan.Stages[0]
	if len(stage0) != 2 || (stage0[0] != "db" && stage0[1] != "db") {
		t.Errorf("unexpected stage 0: %v", stage0)
	}

	// Stage 1 should be backend and worker
	stage1 := plan.Stages[1]
	if len(stage1) != 2 {
		t.Errorf("unexpected stage 1: %v", stage1)
	}

	// Stage 2 should be frontend
	stage2 := plan.Stages[2]
	if len(stage2) != 1 || stage2[0] != "frontend" {
		t.Errorf("unexpected stage 2: %v", stage2)
	}

	// Verify Upstream / Downstream
	backendDeps := g.Upstream("backend")
	if len(backendDeps) != 2 {
		t.Errorf("expected 2 dependencies for backend, got %v", backendDeps)
	}

	redisDependents := g.Downstream("redis")
	// backend and worker depend directly; frontend depends transitively via backend
	if len(redisDependents) != 3 {
		t.Errorf("expected 3 dependents for redis (backend, worker, frontend), got %v", redisDependents)
	}
}

func TestCycleDetection(t *testing.T) {
	// A -> B -> C -> A
	cyclicCfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"svc-a": {
				DependsOn: map[string]compose.DependsOnCondition{
					"svc-b": {Condition: "service_started"},
				},
			},
			"svc-b": {
				DependsOn: map[string]compose.DependsOnCondition{
					"svc-c": {Condition: "service_started"},
				},
			},
			"svc-c": {
				DependsOn: map[string]compose.DependsOnCondition{
					"svc-a": {Condition: "service_started"},
				},
			},
			"standalone": {},
		},
	}

	g := BuildGraph(cyclicCfg)
	cycles := FindCycles(g)

	if len(cycles) == 0 {
		t.Fatalf("expected cycle to be detected, got 0")
	}

	firstCycle := cycles[0]
	if len(firstCycle.Services) != 4 {
		t.Errorf("expected cycle loop of 4 nodes, got %v", firstCycle.Services)
	}

	// Computing startup plan should return error
	_, err := ComputeStartupPlan(g)
	if err == nil {
		t.Errorf("expected error from ComputeStartupPlan on cyclic graph")
	}
}

func TestSubGraphExtraction(t *testing.T) {
	cfg := createTestConfig()
	g := BuildGraph(cfg)

	// Subgraph for backend + upstream only
	sub := g.SubGraph([]string{"backend"}, true, false)
	if len(sub.Nodes) != 3 {
		t.Errorf("expected 3 nodes (backend, db, redis), got %d", len(sub.Nodes))
	}
	if _, ok := sub.Nodes["frontend"]; ok {
		t.Errorf("frontend should not be in upstream subgraph")
	}
	if _, ok := sub.Nodes["worker"]; ok {
		t.Errorf("worker should not be in upstream subgraph")
	}
}

func TestGraphExportFormats(t *testing.T) {
	cfg := createTestConfig()
	g := BuildGraph(cfg)

	mermaid := g.ToMermaid()
	if !strings.Contains(mermaid, "flowchart TD") || !strings.Contains(mermaid, "backend -->|healthy| db") {
		t.Errorf("invalid mermaid output: %s", mermaid)
	}

	dot := g.ToDOT()
	if !strings.Contains(dot, "digraph DockerTopology") {
		t.Errorf("invalid DOT output: %s", dot)
	}

	plan, err := ComputeStartupPlan(g)
	if err != nil {
		t.Fatalf("failed to compute plan: %v", err)
	}
	ascii := plan.ToASCII()
	if !strings.Contains(ascii, "Stage 0:") {
		t.Errorf("invalid ASCII output: %s", ascii)
	}
}
