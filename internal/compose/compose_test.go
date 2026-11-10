package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePortString(t *testing.T) {
	tests := []struct {
		input     string
		target    int
		published int
		protocol  string
		hostIP    string
	}{
		{"80", 80, 0, "tcp", ""},
		{"8080:80", 80, 8080, "tcp", ""},
		{"127.0.0.1:5432:5432", 5432, 5432, "tcp", "127.0.0.1"},
		{"9090:9090/udp", 9090, 9090, "udp", ""},
	}

	for _, tt := range tests {
		pm, err := ParsePortString(tt.input)
		if err != nil {
			t.Fatalf("unexpected error parsing %q: %v", tt.input, err)
		}
		if pm.Target != tt.target || pm.Published != tt.published || pm.Protocol != tt.protocol || pm.HostIP != tt.hostIP {
			t.Errorf("ParsePortString(%q) = %+v, expected target=%d, pub=%d, proto=%s, ip=%s",
				tt.input, pm, tt.target, tt.published, tt.protocol, tt.hostIP)
		}
	}
}

func TestParserVariableInterpolation(t *testing.T) {
	env := map[string]string{
		"APP_PORT": "3000",
		"DB_USER":  "postgres",
	}
	parser := NewParser(env)

	yamlContent := `
services:
  web:
    image: my-app:latest
    ports:
      - "${APP_PORT:-8080}:80"
    environment:
      - DB_USER=${DB_USER}
      - DB_PASS=${DB_PASS:-secret123}
      - ESCAPED=$${DOLLAR_VAR}
      - UNSET_VAR=${MISSING_VAR}
`
	res, err := parser.ParseBytes([]byte(yamlContent), "test-compose.yml")
	if err != nil {
		t.Fatalf("failed to parse bytes: %v", err)
	}

	webSvc, exists := res.Config.Services["web"]
	if !exists {
		t.Fatalf("expected 'web' service to exist")
	}

	if len(webSvc.Ports) != 1 || webSvc.Ports[0].Published != 3000 {
		t.Errorf("expected published port 3000, got %+v", webSvc.Ports)
	}

	if webSvc.Environment["DB_USER"] != "postgres" {
		t.Errorf("expected DB_USER=postgres, got %q", webSvc.Environment["DB_USER"])
	}

	if webSvc.Environment["DB_PASS"] != "secret123" {
		t.Errorf("expected default DB_PASS=secret123, got %q", webSvc.Environment["DB_PASS"])
	}

	if webSvc.Environment["ESCAPED"] != "${DOLLAR_VAR}" {
		t.Errorf("expected escaped ${DOLLAR_VAR}, got %q", webSvc.Environment["ESCAPED"])
	}

	// Verify unset env detection
	foundUnset := false
	for _, u := range res.UnsetEnvs {
		if u == "MISSING_VAR" {
			foundUnset = true
			break
		}
	}
	if !foundUnset {
		t.Errorf("expected MISSING_VAR in UnsetEnvs, got %v", res.UnsetEnvs)
	}
}

func TestMergerBaseAndOverride(t *testing.T) {
	baseYAML := `
services:
  api:
    image: backend:v1
    ports:
      - "8080:8080"
    environment:
      ENV: "production"
      LOG_LEVEL: "info"
    depends_on:
      - db
  db:
    image: postgres:16
`
	overrideYAML := `
services:
  api:
    image: backend:dev
    environment:
      LOG_LEVEL: "debug"
      DEBUG_PORT: "9229"
    ports:
      - "9229:9229"
    depends_on:
      redis:
        condition: service_started
  redis:
    image: redis:7
`
	parser := NewParser(nil)
	baseRes, err := parser.ParseBytes([]byte(baseYAML), "docker-compose.yml")
	if err != nil {
		t.Fatalf("parse base error: %v", err)
	}
	ovRes, err := parser.ParseBytes([]byte(overrideYAML), "docker-compose.override.yml")
	if err != nil {
		t.Fatalf("parse override error: %v", err)
	}

	merger := NewMerger()
	merged := merger.Merge(baseRes.Config, ovRes.Config)

	// api should have 3 services total: api, db, redis
	if len(merged.Services) != 3 {
		t.Fatalf("expected 3 services in merged config, got %d", len(merged.Services))
	}

	apiSvc := merged.Services["api"]
	if apiSvc.Image != "backend:dev" {
		t.Errorf("expected overridden image backend:dev, got %q", apiSvc.Image)
	}
	if apiSvc.Environment["LOG_LEVEL"] != "debug" || apiSvc.Environment["ENV"] != "production" {
		t.Errorf("environment merge failed: %+v", apiSvc.Environment)
	}
	if len(apiSvc.Ports) != 2 {
		t.Errorf("expected 2 ports in api service, got %d", len(apiSvc.Ports))
	}
	if _, ok := apiSvc.DependsOn["db"]; !ok {
		t.Errorf("expected depends_on to preserve 'db'")
	}
	if _, ok := apiSvc.DependsOn["redis"]; !ok {
		t.Errorf("expected depends_on to include 'redis'")
	}
}

func TestMergerProfileFiltering(t *testing.T) {
	yamlText := `
services:
  core:
    image: core:latest
  debug-tool:
    image: debugger:latest
    profiles: ["debug"]
  worker:
    image: worker:latest
    profiles: ["workers", "full"]
`
	parser := NewParser(nil)
	res, err := parser.ParseBytes([]byte(yamlText), "compose.yml")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	merger := NewMerger()

	// Default profile: only core should be active
	defaultFiltered := merger.FilterByProfiles(res.Config, []string{})
	if len(defaultFiltered.Services) != 1 {
		t.Errorf("expected 1 default service, got %d", len(defaultFiltered.Services))
	}
	if _, ok := defaultFiltered.Services["core"]; !ok {
		t.Errorf("expected 'core' to be active")
	}

	// Active profile 'debug': core and debug-tool should be active
	debugFiltered := merger.FilterByProfiles(res.Config, []string{"debug"})
	if len(debugFiltered.Services) != 2 {
		t.Errorf("expected 2 services for 'debug' profile, got %d", len(debugFiltered.Services))
	}

	// Wildcard '*': all services
	allFiltered := merger.FilterByProfiles(res.Config, []string{"*"})
	if len(allFiltered.Services) != 3 {
		t.Errorf("expected 3 services with '*', got %d", len(allFiltered.Services))
	}
}

func TestParserLoadDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	dotEnvPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(dotEnvPath, []byte("TEST_KEY=\"hello_world\"\n# comment\nPORT=5000\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp .env: %v", err)
	}

	parser := NewParser(nil)
	if err := parser.LoadDotEnv(dotEnvPath); err != nil {
		t.Fatalf("failed to load .env: %v", err)
	}

	if parser.envMap["TEST_KEY"] != "hello_world" {
		t.Errorf("expected TEST_KEY=hello_world, got %q", parser.envMap["TEST_KEY"])
	}
	if parser.envMap["PORT"] != "5000" {
		t.Errorf("expected PORT=5000, got %q", parser.envMap["PORT"])
	}
}
