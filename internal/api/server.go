package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/stackloom/internal/compose"
	"github.com/alexandrmotologa/stackloom/internal/config"
	"github.com/alexandrmotologa/stackloom/internal/docker"
	"github.com/alexandrmotologa/stackloom/internal/export"
	"github.com/alexandrmotologa/stackloom/internal/graph"
	"github.com/alexandrmotologa/stackloom/internal/linter"
	"github.com/gorilla/websocket"
)

// Server coordinates the REST and WebSocket endpoints for StackLoom.
type Server struct {
	cfg            *config.Config
	parser         *compose.Parser
	merger         *compose.Merger
	linter         *linter.Linter
	dockerClient   *docker.Client
	basePath       string
	mu             sync.RWMutex
	rawCompose     *compose.ComposeConfig
	cachedCompose  *compose.ComposeConfig
	cachedGraph    *graph.Graph
	cachedPlan     *graph.StartupPlan
	cachedLinter   *linter.Result
	allProfiles    []string
	activeProfiles []string
	unsetEnvs      []string
	upgrader       websocket.Upgrader
}

// NewServer initializes the API server with parsed compose state.
func NewServer(cfg *config.Config, dockerClient *docker.Client) (*Server, error) {
	s := &Server{
		cfg:          cfg,
		parser:       compose.NewParser(nil),
		merger:       compose.NewMerger(),
		linter:       linter.NewLinter(cfg.ProbeHostSockets),
		dockerClient: dockerClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all local origins
			},
		},
		activeProfiles: cfg.ActiveProfiles,
	}

	if err := s.Reload(); err != nil {
		return nil, err
	}

	return s, nil
}

// Reload parses all configured Compose files and recomputes the topology and linter.
func (s *Server) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.cfg.ComposeFiles) == 0 {
		s.cfg.ComposeFiles = []string{"docker-compose.yml"}
	}

	var baseConfig *compose.ComposeConfig
	var allUnsets []string
	baseFile := s.cfg.ComposeFiles[0]
	s.basePath = filepath.Dir(baseFile)

	for i, file := range s.cfg.ComposeFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			if i > 0 {
				// Optional overrides may not exist
				continue
			}
			return fmt.Errorf("compose file %s not found", file)
		}

		res, err := s.parser.ParseFile(file)
		if err != nil {
			return fmt.Errorf("error parsing %s: %w", file, err)
		}

		allUnsets = append(allUnsets, res.UnsetEnvs...)
		if baseConfig == nil {
			baseConfig = res.Config
		} else {
			baseConfig = s.merger.Merge(baseConfig, res.Config)
		}
	}

	// Auto-check for docker-compose.override.yml if not already included
	overridePath := filepath.Join(s.basePath, "docker-compose.override.yml")
	if _, err := os.Stat(overridePath); err == nil {
		included := false
		for _, f := range s.cfg.ComposeFiles {
			if filepath.Clean(f) == filepath.Clean(overridePath) {
				included = true
				break
			}
		}
		if !included {
			res, err := s.parser.ParseFile(overridePath)
			if err == nil {
				allUnsets = append(allUnsets, res.UnsetEnvs...)
				baseConfig = s.merger.Merge(baseConfig, res.Config)
			}
		}
	}

	s.rawCompose = baseConfig
	s.allProfiles = s.merger.GetAllProfiles(baseConfig)
	s.unsetEnvs = allUnsets

	// Filter by active profiles
	filtered := s.merger.FilterByProfiles(baseConfig, s.activeProfiles)
	s.cachedCompose = filtered

	// Build DAG
	g := graph.BuildGraph(filtered)
	s.cachedGraph = g

	// Compute startup order (if acyclic)
	plan, _ := graph.ComputeStartupPlan(g)
	s.cachedPlan = plan

	// Run Linter
	linterResult := s.linter.Lint(filtered, g, s.unsetEnvs, s.basePath)
	s.cachedLinter = linterResult

	return nil
}

// RegisterRoutes registers all API endpoints and static assets.
func (s *Server) RegisterRoutes(mux *http.ServeMux, staticFS http.FileSystem) {
	// REST API
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/topology", s.handleTopology)
	mux.HandleFunc("/api/lint", s.handleLint)
	mux.HandleFunc("/api/containers", s.handleContainers)
	mux.HandleFunc("/api/profiles/active", s.handleActiveProfiles)
	mux.HandleFunc("/api/actions/service", s.handleServiceAction)
	mux.HandleFunc("/api/actions/subgraph", s.handleSubGraphAction)
	mux.HandleFunc("/api/export", s.handleExportHTML)

	// WebSockets
	mux.HandleFunc("/ws/stats", s.handleWSStats)
	mux.HandleFunc("/ws/logs", s.handleWSLogs)

	// Static SPA Serving
	if staticFS != nil {
		fileServer := http.FileServer(staticFS)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
				http.NotFound(w, r)
				return
			}
			// Check if file exists in staticFS, otherwise fallback to index.html
			f, err := staticFS.Open(r.URL.Path)
			if err != nil {
				// Fallback to index.html for SPA routing
				r.URL.Path = "/"
			} else {
				_ = f.Close()
			}
			fileServer.ServeHTTP(w, r)
		})
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dockerConn := false
	dockerHost := ""
	dockerErr := ""
	if s.dockerClient != nil {
		dockerConn = s.dockerClient.IsConnected()
		dockerHost = s.dockerClient.Host()
		dockerErr = s.dockerClient.LastError()
	}

	resp := map[string]interface{}{
		"docker_connected": dockerConn,
		"docker_host":      dockerHost,
		"docker_error":     dockerErr,
		"compose_files":    s.cfg.ComposeFiles,
		"all_profiles":     s.allProfiles,
		"active_profiles":  s.activeProfiles,
		"project_name":     s.rawCompose.Name,
		"service_count":    len(s.cachedCompose.Services),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := map[string]interface{}{
		"graph":           s.cachedGraph,
		"startup_plan":    s.cachedPlan,
		"all_profiles":    s.allProfiles,
		"active_profiles": s.activeProfiles,
		"compose":         s.cachedCompose,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLint(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	writeJSON(w, http.StatusOK, s.cachedLinter)
}

func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	if s.dockerClient == nil || !s.dockerClient.IsConnected() {
		writeJSON(w, http.StatusOK, []docker.ContainerInfo{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	s.mu.RLock()
	projectName := s.rawCompose.Name
	s.mu.RUnlock()

	containers, err := s.dockerClient.ListContainers(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, containers)
}

type activeProfilesRequest struct {
	Profiles []string `json:"profiles"`
}

func (s *Server) handleActiveProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req activeProfilesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	s.mu.Lock()
	s.activeProfiles = req.Profiles
	s.mu.Unlock()

	if err := s.Reload(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	resp := map[string]interface{}{
		"active_profiles": s.activeProfiles,
		"service_count":   len(s.cachedCompose.Services),
		"graph":           s.cachedGraph,
		"startup_plan":    s.cachedPlan,
	}
	writeJSON(w, http.StatusOK, resp)
}

type serviceActionRequest struct {
	Service string `json:"service"`
	Action  string `json:"action"` // "start", "stop", "restart"
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req serviceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if s.dockerClient == nil || !s.dockerClient.IsConnected() {
		writeError(w, http.StatusServiceUnavailable, "Docker socket is not connected")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	s.mu.RLock()
	projectName := s.rawCompose.Name
	s.mu.RUnlock()

	containers, err := s.dockerClient.ListContainers(ctx, projectName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var targetContainer *docker.ContainerInfo
	for _, c := range containers {
		if strings.EqualFold(c.Service, req.Service) {
			targetContainer = &c
			break
		}
	}

	if targetContainer == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("No running or stopped container found for service %q", req.Service))
		return
	}

	var actionErr error
	switch req.Action {
	case "start":
		actionErr = s.dockerClient.StartContainer(ctx, targetContainer.ID)
	case "stop":
		actionErr = s.dockerClient.StopContainer(ctx, targetContainer.ID)
	case "restart":
		actionErr = s.dockerClient.RestartContainer(ctx, targetContainer.ID)
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Unknown action %q", req.Action))
		return
	}

	if actionErr != nil {
		writeError(w, http.StatusInternalServerError, actionErr.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"service": req.Service,
		"action":  req.Action,
	})
}

type subGraphActionRequest struct {
	Services []string `json:"services"`
	Action   string   `json:"action"` // "start", "stop", "restart"
	Upstream bool     `json:"upstream"`
}

func (s *Server) handleSubGraphAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req subGraphActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	s.mu.RLock()
	g := s.cachedGraph
	s.mu.RUnlock()

	if g == nil {
		writeError(w, http.StatusInternalServerError, "Topology graph not loaded")
		return
	}

	// Calculate sub-graph
	sub := g.SubGraph(req.Services, req.Upstream, false)
	plan, err := graph.ComputeStartupPlan(sub)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Cannot order sub-graph: %v", err))
		return
	}

	order := plan.Order
	if req.Action == "stop" {
		order = plan.ShutdownOrder
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"action":   req.Action,
		"subgraph": order,
		"stages":   plan.Stages,
	})
}

func (s *Server) handleExportHTML(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	html, err := export.GenerateHTML(s.cachedCompose, s.cachedGraph, s.cachedPlan, s.cachedLinter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

func (s *Server) handleWSStats(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.RLock()
		projectName := s.rawCompose.Name
		s.mu.RUnlock()

		statsPayload := map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"services":  make(map[string]interface{}),
		}

		servicesMap := statsPayload["services"].(map[string]interface{})

		if s.dockerClient != nil && s.dockerClient.IsConnected() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			containers, err := s.dockerClient.ListContainers(ctx, projectName)
			cancel()

			if err == nil {
				for _, c := range containers {
					if c.Service == "" || c.State != docker.StateRunning {
						continue
					}
					statCtx, statCancel := context.WithTimeout(context.Background(), 1*time.Second)
					metrics, err := s.dockerClient.GetStats(statCtx, c.ID)
					statCancel()

					if err == nil && metrics != nil {
						servicesMap[c.Service] = map[string]interface{}{
							"cpu_percentage":   metrics.CPUPercentage,
							"memory_usage_mb":  float64(metrics.MemoryUsage) / (1024 * 1024),
							"memory_limit_mb":  float64(metrics.MemoryLimit) / (1024 * 1024),
							"memory_percent":   metrics.MemoryPercent,
							"state":            c.State,
							"health":           c.Health,
						}
					}
				}
			}
		}

		if err := conn.WriteJSON(statsPayload); err != nil {
			return
		}
	}
}

func (s *Server) handleWSLogs(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	if service == "" {
		http.Error(w, "missing 'service' query parameter", http.StatusBadRequest)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	if s.dockerClient == nil || !s.dockerClient.IsConnected() {
		_ = conn.WriteJSON(docker.LogLine{
			Service:   service,
			Stream:    docker.StreamStderr,
			Timestamp: time.Now(),
			Message:   "Docker socket is not connected. Logs unavailable in offline mode.",
		})
		return
	}

	s.mu.RLock()
	projectName := s.rawCompose.Name
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	containers, err := s.dockerClient.ListContainers(ctx, projectName)
	cancel()

	if err != nil {
		_ = conn.WriteJSON(docker.LogLine{
			Service:   service,
			Stream:    docker.StreamStderr,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Failed to list containers: %v", err),
		})
		return
	}

	var targetContainer *docker.ContainerInfo
	for _, c := range containers {
		if strings.EqualFold(c.Service, service) {
			targetContainer = &c
			break
		}
	}

	if targetContainer == nil {
		_ = conn.WriteJSON(docker.LogLine{
			Service:   service,
			Stream:    docker.StreamStderr,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("No active container found for service %q", service),
		})
		return
	}

	logCtx, logCancel := context.WithCancel(r.Context())
	defer logCancel()

	linesCh, errCh := s.dockerClient.StreamLogs(logCtx, targetContainer.ID, 100, true)

	for {
		select {
		case line, ok := <-linesCh:
			if !ok {
				return
			}
			line.Service = service
			if err := conn.WriteJSON(line); err != nil {
				return
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				_ = conn.WriteJSON(docker.LogLine{
					Service:   service,
					Stream:    docker.StreamStderr,
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("Log stream error: %v", err),
				})
			}
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
