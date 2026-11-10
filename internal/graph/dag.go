package graph

import (
	"sort"

	"github.com/alexandrmotologa/stackloom/internal/compose"
)

// DependencyCondition defines when a dependent container starts.
type DependencyCondition string

const (
	ConditionStarted   DependencyCondition = "service_started"
	ConditionHealthy   DependencyCondition = "service_healthy"
	ConditionCompleted DependencyCondition = "service_completed_successfully"
)

// EdgeType defines how the dependency relationship is expressed.
type EdgeType string

const (
	EdgeExplicit    EdgeType = "depends_on"
	EdgeNetworkMode EdgeType = "network_mode"
	EdgeVolumeFrom  EdgeType = "volumes_from"
)

// Edge models a directed connection from a dependent service to its dependency.
type Edge struct {
	From      string              `json:"from"`      // Dependent service
	To        string              `json:"to"`        // Dependency service
	Condition DependencyCondition `json:"condition"` // Required state of dependency
	Type      EdgeType            `json:"type"`      // Dependency relationship type
}

// Node holds metadata and configuration for a service vertex in the graph.
type Node struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Image         string                `json:"image,omitempty"`
	ContainerName string                `json:"container_name,omitempty"`
	Ports         []compose.PortMapping `json:"ports,omitempty"`
	Networks      []string              `json:"networks,omitempty"`
	Volumes       []string              `json:"volumes,omitempty"`
	Profiles      []string              `json:"profiles,omitempty"`
	HasHealthCheck bool                 `json:"has_healthcheck"`
	User          string                `json:"user,omitempty"`
	Privileged    bool                  `json:"privileged"`
	InDegree      int                   `json:"in_degree"`  // Number of dependencies
	OutDegree     int                   `json:"out_degree"` // Number of dependents
}

// Graph models the entire service dependency network.
type Graph struct {
	Nodes  map[string]*Node    `json:"nodes"`
	Edges  []*Edge             `json:"edges"`
	Adj    map[string][]string `json:"-"` // u -> services u depends on
	RevAdj map[string][]string `json:"-"` // u -> services that depend on u
}

// NewGraph creates an empty graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes:  make(map[string]*Node),
		Edges:  make([]*Edge, 0),
		Adj:    make(map[string][]string),
		RevAdj: make(map[string][]string),
	}
}

// BuildGraph constructs a directed dependency graph from a ComposeConfig.
func BuildGraph(cfg *compose.ComposeConfig) *Graph {
	g := NewGraph()
	if cfg == nil {
		return g
	}

	// 1. Initialize nodes
	for name, s := range cfg.Services {
		var vols []string
		for _, v := range s.Volumes {
			vols = append(vols, v.Raw)
		}

		node := &Node{
			ID:             name,
			Name:           name,
			Image:          s.Image,
			ContainerName:  s.ContainerName,
			Ports:          s.Ports,
			Networks:       s.Networks,
			Volumes:        vols,
			Profiles:       s.Profiles,
			HasHealthCheck: s.HealthCheck != nil && !s.HealthCheck.Disable,
			User:           s.User,
			Privileged:     s.Privileged,
		}
		g.Nodes[name] = node
		g.Adj[name] = make([]string, 0)
		g.RevAdj[name] = make([]string, 0)
	}

	// 2. Discover edges
	edgeMap := make(map[string]*Edge)
	for name, s := range cfg.Services {
		// Explicit depends_on
		for dep, cond := range s.DependsOn {
			condition := DependencyCondition(cond.Condition)
			if condition == "" {
				condition = ConditionStarted
			}
			addEdge(g, edgeMap, name, dep, condition, EdgeExplicit)
		}

		// network_mode: "service:xxx"
		if len(s.NetworkMode) > 8 && s.NetworkMode[:8] == "service:" {
			target := s.NetworkMode[8:]
			addEdge(g, edgeMap, name, target, ConditionStarted, EdgeNetworkMode)
		}
	}

	// Compute in/out degrees
	for _, e := range g.Edges {
		if n, ok := g.Nodes[e.From]; ok {
			n.InDegree++
		}
		if n, ok := g.Nodes[e.To]; ok {
			n.OutDegree++
		}
	}

	return g
}

func addEdge(g *Graph, edgeMap map[string]*Edge, from, to string, condition DependencyCondition, edgeType EdgeType) {
	key := from + "->" + to
	if _, exists := edgeMap[key]; exists {
		return
	}
	edge := &Edge{
		From:      from,
		To:        to,
		Condition: condition,
		Type:      edgeType,
	}
	edgeMap[key] = edge
	g.Edges = append(g.Edges, edge)
	g.Adj[from] = append(g.Adj[from], to)
	g.RevAdj[to] = append(g.RevAdj[to], from)
}

// Upstream returns all direct and transitive dependencies of a given service.
func (g *Graph) Upstream(service string) []string {
	visited := make(map[string]bool)
	var queue []string
	queue = append(queue, g.Adj[service]...)

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if visited[curr] {
			continue
		}
		visited[curr] = true
		for _, next := range g.Adj[curr] {
			if !visited[next] {
				queue = append(queue, next)
			}
		}
	}

	var result []string
	for k := range visited {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// Downstream returns all services that directly or transitively depend on a given service.
func (g *Graph) Downstream(service string) []string {
	visited := make(map[string]bool)
	var queue []string
	queue = append(queue, g.RevAdj[service]...)

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if visited[curr] {
			continue
		}
		visited[curr] = true
		for _, next := range g.RevAdj[curr] {
			if !visited[next] {
				queue = append(queue, next)
			}
		}
	}

	var result []string
	for k := range visited {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// SubGraph creates a filtered graph containing only the specified services and optionally their upstream/downstream relatives.
func (g *Graph) SubGraph(services []string, includeUpstream, includeDownstream bool) *Graph {
	selected := make(map[string]bool)
	for _, s := range services {
		selected[s] = true
		if includeUpstream {
			for _, up := range g.Upstream(s) {
				selected[up] = true
			}
		}
		if includeDownstream {
			for _, down := range g.Downstream(s) {
				selected[down] = true
			}
		}
	}

	sub := NewGraph()
	for s := range selected {
		if node, ok := g.Nodes[s]; ok {
			sub.Nodes[s] = &Node{
				ID:             node.ID,
				Name:           node.Name,
				Image:          node.Image,
				ContainerName:  node.ContainerName,
				Ports:          node.Ports,
				Networks:       node.Networks,
				Volumes:        node.Volumes,
				Profiles:       node.Profiles,
				HasHealthCheck: node.HasHealthCheck,
				User:           node.User,
				Privileged:     node.Privileged,
			}
			sub.Adj[s] = make([]string, 0)
			sub.RevAdj[s] = make([]string, 0)
		}
	}

	edgeMap := make(map[string]*Edge)
	for _, e := range g.Edges {
		if selected[e.From] && selected[e.To] {
			addEdge(sub, edgeMap, e.From, e.To, e.Condition, e.Type)
		}
	}

	for _, e := range sub.Edges {
		if n, ok := sub.Nodes[e.From]; ok {
			n.InDegree++
		}
		if n, ok := sub.Nodes[e.To]; ok {
			n.OutDegree++
		}
	}

	return sub
}
