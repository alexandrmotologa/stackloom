package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Client handles communication with the Docker Engine REST API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	host       string
	mu         sync.RWMutex
	connected  bool
	lastError  string
}

// NewClient initializes a Docker client with the specified host or auto-discovers from environment.
func NewClient(customHost string) (*Client, error) {
	host := customHost
	if host == "" {
		host = os.Getenv("DOCKER_HOST")
	}
	if host == "" {
		host = DefaultDockerHost
	}

	httpClient, baseURL, err := createHTTPClient(host)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Docker transport: %w", err)
	}

	c := &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		host:       host,
	}

	// Initial probe with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx); err == nil {
		c.connected = true
	} else {
		c.connected = false
		c.lastError = err.Error()
	}

	return c, nil
}

// Host returns the configured Docker connection URI.
func (c *Client) Host() string {
	return c.host
}

// IsConnected returns whether the Docker daemon is actively reachable.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// LastError returns the most recent connection failure message, if any.
func (c *Client) LastError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}

// Ping verifies Docker daemon responsiveness via GET /_ping.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/_ping", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.mu.Lock()
		c.connected = false
		c.lastError = err.Error()
		c.mu.Unlock()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.mu.Lock()
		c.connected = false
		c.lastError = fmt.Sprintf("Docker returned status %d", resp.StatusCode)
		c.mu.Unlock()
		return fmt.Errorf("ping returned HTTP %d", resp.StatusCode)
	}

	c.mu.Lock()
	c.connected = true
	c.lastError = ""
	c.mu.Unlock()
	return nil
}

// rawContainerJSON models the Docker Engine GET /containers/json response item.
type rawContainerJSON struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Created int64             `json:"Created"`
	Labels  map[string]string `json:"Labels"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
}

// ListContainers retrieves all compose-managed containers, optionally filtered by project name.
func (c *Client) ListContainers(ctx context.Context, projectName string) ([]ContainerInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v1.41/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker list failed (status %d): %s", resp.StatusCode, string(body))
	}

	var rawList []rawContainerJSON
	if err := json.NewDecoder(resp.Body).Decode(&rawList); err != nil {
		return nil, err
	}

	var result []ContainerInfo
	for _, raw := range rawList {
		service := raw.Labels["com.docker.compose.service"]
		project := raw.Labels["com.docker.compose.project"]

		if projectName != "" && project != "" && !strings.EqualFold(project, projectName) {
			continue
		}

		// Clean container names (strip leading /)
		var names []string
		for _, n := range raw.Names {
			names = append(names, strings.TrimPrefix(n, "/"))
		}

		var ports []PortDescription
		for _, p := range raw.Ports {
			ports = append(ports, PortDescription{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}

		// Infer health status from Status string (e.g. "Up 2 hours (healthy)")
		health := "none"
		statusLower := strings.ToLower(raw.Status)
		if strings.Contains(statusLower, "(healthy)") {
			health = "healthy"
		} else if strings.Contains(statusLower, "(unhealthy)") {
			health = "unhealthy"
		} else if strings.Contains(statusLower, "(health: starting)") {
			health = "starting"
		}

		result = append(result, ContainerInfo{
			ID:        raw.ID,
			Names:     names,
			Image:     raw.Image,
			Service:   service,
			Project:   project,
			State:     ContainerState(raw.State),
			Status:    raw.Status,
			Health:    health,
			StartedAt: time.Unix(raw.Created, 0),
			Ports:     ports,
			Labels:    raw.Labels,
		})
	}

	return result, nil
}

// rawDockerStats models the GET /containers/{id}/stats response.
type rawDockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
		Stats struct {
			Cache uint64 `json:"cache"`
		} `json:"stats"`
	} `json:"memory_stats"`
}

// GetStats reads instantaneous CPU and memory metrics for a container.
func (c *Client) GetStats(ctx context.Context, containerID string) (*ContainerMetrics, error) {
	url := fmt.Sprintf("%s/v1.41/containers/%s/stats?stream=false", c.baseURL, containerID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stats failed with status %d", resp.StatusCode)
	}

	var raw rawDockerStats
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	// Calculate CPU percentage
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage) - float64(raw.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(raw.CPUStats.SystemCPUUsage) - float64(raw.PreCPUStats.SystemCPUUsage)
	onlineCPUs := float64(raw.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(raw.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 {
		onlineCPUs = 1.0
	}

	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	// Calculate Memory
	memUsage := raw.MemoryStats.Usage
	if raw.MemoryStats.Stats.Cache > 0 && memUsage > raw.MemoryStats.Stats.Cache {
		memUsage -= raw.MemoryStats.Stats.Cache
	}
	memLimit := raw.MemoryStats.Limit
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (float64(memUsage) / float64(memLimit)) * 100.0
	}

	return &ContainerMetrics{
		ContainerID:   containerID,
		Timestamp:     time.Now(),
		CPUPercentage: cpuPercent,
		MemoryUsage:   memUsage,
		MemoryLimit:   memLimit,
		MemoryPercent: memPercent,
	}, nil
}

// StartContainer issues POST /containers/{id}/start.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	url := fmt.Sprintf("%s/v1.41/containers/%s/start", c.baseURL, containerID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to start container %s (status %d): %s", containerID, resp.StatusCode, string(body))
	}
	return nil
}

// StopContainer issues POST /containers/{id}/stop.
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	url := fmt.Sprintf("%s/v1.41/containers/%s/stop?t=10", c.baseURL, containerID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to stop container %s (status %d): %s", containerID, resp.StatusCode, string(body))
	}
	return nil
}

// RestartContainer issues POST /containers/{id}/restart.
func (c *Client) RestartContainer(ctx context.Context, containerID string) error {
	url := fmt.Sprintf("%s/v1.41/containers/%s/restart?t=10", c.baseURL, containerID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to restart container %s (status %d): %s", containerID, resp.StatusCode, string(body))
	}
	return nil
}
