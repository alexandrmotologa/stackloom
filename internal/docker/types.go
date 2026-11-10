package docker

import "time"

// ContainerState represents runtime health and status.
type ContainerState string

const (
	StateRunning    ContainerState = "running"
	StateExited     ContainerState = "exited"
	StateRestarting ContainerState = "restarting"
	StateCreated    ContainerState = "created"
	StateDead       ContainerState = "dead"
	StatePaused     ContainerState = "paused"
	StateNotFound   ContainerState = "not_found"
)

// ContainerInfo models summary details for an active or stopped container.
type ContainerInfo struct {
	ID          string            `json:"id"`
	Names       []string          `json:"names"`
	Image       string            `json:"image"`
	Service     string            `json:"service"`
	Project     string            `json:"project"`
	State       ContainerState    `json:"state"`
	Status      string            `json:"status"`
	Health      string            `json:"health,omitempty"` // "healthy", "unhealthy", "starting", "none"
	ExitCode    int               `json:"exit_code"`
	StartedAt   time.Time         `json:"started_at"`
	Ports       []PortDescription `json:"ports"`
	Labels      map[string]string `json:"labels"`
}

// PortDescription models published and exposed ports reported by Docker Engine.
type PortDescription struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port,omitempty"`
	Type        string `json:"type"` // tcp or udp
}

// ContainerMetrics holds calculated CPU and memory statistics.
type ContainerMetrics struct {
	ContainerID   string    `json:"container_id"`
	Service       string    `json:"service"`
	Timestamp     time.Time `json:"timestamp"`
	CPUPercentage float64   `json:"cpu_percentage"`
	MemoryUsage   uint64    `json:"memory_usage_bytes"`
	MemoryLimit   uint64    `json:"memory_limit_bytes"`
	MemoryPercent float64   `json:"memory_percentage"`
}

// LogStream specifies stdout vs stderr.
type LogStream string

const (
	StreamStdout LogStream = "stdout"
	StreamStderr LogStream = "stderr"
)

// LogLine models a demultiplexed log entry with timestamp.
type LogLine struct {
	Service   string    `json:"service"`
	Stream    LogStream `json:"stream"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}
