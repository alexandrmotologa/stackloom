package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandrmotologa/stackloom/internal/config"
)

func createTestComposeFile(t *testing.T) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "docker-compose.yml")
	content := `
services:
  db:
    image: postgres:16
    ports:
      - "5432:5432"
  redis:
    image: redis:7
  api:
    image: backend:latest
    ports:
      - "8080:8080"
    depends_on:
      - db
      - redis
  worker:
    image: worker:latest
    profiles: ["background"]
    depends_on:
      - redis
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test compose file: %v", err)
	}
	return filePath
}

func TestServerRESTEndpoints(t *testing.T) {
	composePath := createTestComposeFile(t)

	cfg := config.DefaultConfig()
	cfg.ComposeFiles = []string{composePath}
	cfg.ProbeHostSockets = false

	server, err := NewServer(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux, nil)

	// 1. GET /api/status
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status endpoint returned HTTP %d", w.Code)
	}
	var statusResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if statusResp["docker_connected"] != false {
		t.Errorf("expected docker_connected to be false")
	}

	// 2. GET /api/topology
	req = httptest.NewRequest("GET", "/api/topology", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("topology endpoint returned HTTP %d", w.Code)
	}
	var topoResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &topoResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	graphObj, ok := topoResp["graph"].(map[string]interface{})
	if !ok {
		t.Fatalf("graph object missing from topology response")
	}
	nodes, ok := graphObj["nodes"].(map[string]interface{})
	if !ok || len(nodes) != 3 {
		// default profile should have 3 services (db, redis, api; worker is in "background" profile)
		t.Errorf("expected 3 default services, got %d", len(nodes))
	}

	// 3. GET /api/lint
	req = httptest.NewRequest("GET", "/api/lint", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("lint endpoint returned HTTP %d", w.Code)
	}

	// 4. POST /api/profiles/active -> activate "background"
	profileReqBody, _ := json.Marshal(map[string]interface{}{
		"profiles": []string{"background"},
	})
	req = httptest.NewRequest("POST", "/api/profiles/active", bytes.NewReader(profileReqBody))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("active profiles endpoint returned HTTP %d", w.Code)
	}
	var activeResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &activeResp)
	if activeResp["service_count"] != float64(4) {
		t.Errorf("expected 4 services with background profile active, got %v", activeResp["service_count"])
	}

	// 5. POST /api/actions/subgraph
	subBody, _ := json.Marshal(map[string]interface{}{
		"services": []string{"api"},
		"action":   "start",
		"upstream": true,
	})
	req = httptest.NewRequest("POST", "/api/actions/subgraph", bytes.NewReader(subBody))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("subgraph action returned HTTP %d", w.Code)
	}
	var subResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &subResp)
	if subResp["success"] != true {
		t.Errorf("expected success: true in subgraph response")
	}

	// 6. GET /api/export
	req = httptest.NewRequest("GET", "/api/export", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("export endpoint returned HTTP %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<!DOCTYPE html>") {
		t.Errorf("expected HTML export document")
	}
}
