package linter

import (
	"net"
	"strconv"
	"testing"

	"github.com/alexandrmotologa/stackloom/internal/compose"
	"github.com/alexandrmotologa/stackloom/internal/graph"
)

func TestPortCollisionInternal(t *testing.T) {
	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"api": {
				Ports: []compose.PortMapping{
					{Target: 8080, Published: 8080, Protocol: "tcp", Raw: "8080:8080"},
				},
			},
			"admin": {
				Ports: []compose.PortMapping{
					{Target: 3000, Published: 8080, Protocol: "tcp", Raw: "8080:3000"},
				},
			},
		},
	}
	g := graph.BuildGraph(cfg)
	l := NewLinter(false)

	res := l.Lint(cfg, g, nil, "")
	if res.Passed {
		t.Errorf("expected linter to fail due to port collision")
	}

	found := false
	for _, f := range res.Findings {
		if f.RuleID == "ERR_PORT_COLLISION" {
			found = true
			if f.SuggestedOverride == "" {
				t.Errorf("expected SuggestedOverride to be populated")
			}
			break
		}
	}
	if !found {
		t.Errorf("ERR_PORT_COLLISION finding not found in %+v", res.Findings)
	}
}

func TestPortCollisionHostSocketProbe(t *testing.T) {
	// Start an actual TCP listener on a dynamic port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	port, _ := strconv.Atoi(portStr)

	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"test-service": {
				Ports: []compose.PortMapping{
					{Target: 80, Published: port, Protocol: "tcp", Raw: strconv.Itoa(port) + ":80"},
				},
			},
		},
	}
	g := graph.BuildGraph(cfg)
	l := NewLinter(true) // Enable host socket probe

	res := l.Lint(cfg, g, nil, "")
	found := false
	for _, f := range res.Findings {
		if f.RuleID == "ERR_PORT_COLLISION" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected host socket probe to detect port %d in use", port)
	}
}

func TestMissingDependency(t *testing.T) {
	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"api": {
				DependsOn: map[string]compose.DependsOnCondition{
					"ghost-db": {Condition: "service_started"},
				},
			},
		},
	}
	g := graph.BuildGraph(cfg)
	l := NewLinter(false)

	res := l.Lint(cfg, g, nil, "")
	if res.Passed {
		t.Errorf("expected linter to fail for missing dependency")
	}

	found := false
	for _, f := range res.Findings {
		if f.RuleID == "ERR_MISSING_DEPENDENCY" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ERR_MISSING_DEPENDENCY finding not found")
	}
}

func TestHealthCheckDependencyWarning(t *testing.T) {
	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"db": {
				Image: "postgres:16",
				// No healthcheck
			},
			"api": {
				DependsOn: map[string]compose.DependsOnCondition{
					"db": {Condition: "service_healthy"},
				},
			},
		},
	}
	g := graph.BuildGraph(cfg)
	l := NewLinter(false)

	res := l.Lint(cfg, g, nil, "")
	found := false
	for _, f := range res.Findings {
		if f.RuleID == "WARN_HEALTHCHECK_MISS" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("WARN_HEALTHCHECK_MISS finding not found")
	}
}

func TestUnsetEnvsWarning(t *testing.T) {
	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"api": {},
		},
	}
	g := graph.BuildGraph(cfg)
	l := NewLinter(false)

	res := l.Lint(cfg, g, []string{"DATABASE_PASSWORD", "API_SECRET"}, "")
	foundCount := 0
	for _, f := range res.Findings {
		if f.RuleID == "WARN_UNSET_ENV_VAR" {
			foundCount++
		}
	}
	if foundCount != 2 {
		t.Errorf("expected 2 WARN_UNSET_ENV_VAR findings, got %d", foundCount)
	}
}

func TestMissingBindMount(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"app": {
				Volumes: []compose.VolumeMapping{
					{Type: "bind", Source: "./non_existent_folder", Target: "/app/data"},
				},
			},
		},
	}
	g := graph.BuildGraph(cfg)
	l := NewLinter(false)

	res := l.Lint(cfg, g, nil, tmpDir)
	found := false
	for _, f := range res.Findings {
		if f.RuleID == "WARN_MISSING_BIND_MOUNT" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WARN_MISSING_BIND_MOUNT finding")
	}
}

func TestSecurityChecks(t *testing.T) {
	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"insecure-app": {
				Privileged:  true,
				User:        "root",
				NetworkMode: "host",
				Volumes: []compose.VolumeMapping{
					{Type: "bind", Source: "/var/run/docker.sock", Target: "/var/run/docker.sock"},
				},
			},
		},
	}
	g := graph.BuildGraph(cfg)
	l := NewLinter(false)

	res := l.Lint(cfg, g, nil, "")
	secRules := make(map[string]bool)
	for _, f := range res.Findings {
		if f.Severity == SeveritySecurity {
			secRules[f.RuleID] = true
		}
	}

	expected := []string{"SEC_PRIVILEGED", "SEC_ROOT_USER", "SEC_DOCKER_SOCKET", "SEC_HOST_NETWORK"}
	for _, exp := range expected {
		if !secRules[exp] {
			t.Errorf("expected security rule %s was not triggered", exp)
		}
	}
}

func TestNetworkIsolationWarning(t *testing.T) {
	cfg := &compose.ComposeConfig{
		Services: map[string]compose.ServiceConfig{
			"frontend": {
				Networks: []string{"public-net"},
				DependsOn: map[string]compose.DependsOnCondition{
					"isolated-db": {Condition: "service_started"},
				},
			},
			"isolated-db": {
				Networks: []string{"private-net"},
			},
		},
	}
	g := graph.BuildGraph(cfg)
	l := NewLinter(false)

	res := l.Lint(cfg, g, nil, "")
	found := false
	for _, f := range res.Findings {
		if f.RuleID == "WARN_NETWORK_ISOLATION" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WARN_NETWORK_ISOLATION finding")
	}
}
