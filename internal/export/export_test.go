package export

import (
	"strings"
	"testing"

	"github.com/alexandrmotologa/stackloom/internal/compose"
	"github.com/alexandrmotologa/stackloom/internal/graph"
	"github.com/alexandrmotologa/stackloom/internal/linter"
)

func TestGenerateHTML(t *testing.T) {
	cfg := &compose.ComposeConfig{
		Name: "test-project",
		Services: map[string]compose.ServiceConfig{
			"db": {Image: "postgres:16"},
			"api": {
				Image: "api:v1",
				DependsOn: map[string]compose.DependsOnCondition{
					"db": {Condition: "service_started"},
				},
			},
		},
	}
	g := graph.BuildGraph(cfg)
	plan, err := graph.ComputeStartupPlan(g)
	if err != nil {
		t.Fatalf("startup plan error: %v", err)
	}

	l := linter.NewLinter(false)
	linterRes := l.Lint(cfg, g, nil, "")

	htmlContent, err := GenerateHTML(cfg, g, plan, linterRes)
	if err != nil {
		t.Fatalf("GenerateHTML failed: %v", err)
	}

	if !strings.Contains(htmlContent, "StackLoom - test-project") {
		t.Errorf("expected title with test-project in HTML")
	}
	if !strings.Contains(htmlContent, "test-project") {
		t.Errorf("expected serialized data in HTML")
	}
	if !strings.Contains(htmlContent, "Topology DAG") {
		t.Errorf("expected UI navigation in HTML")
	}
}
