package graph

import (
	"fmt"
	"sort"
	"strings"
)

// StartupPlan holds execution stages for launching services in waves.
type StartupPlan struct {
	Stages       [][]string `json:"stages"`         // Stage 0 -> Stage 1 -> Stage 2
	Order        []string   `json:"order"`          // Flattened sequential order
	ShutdownOrder []string  `json:"shutdown_order"` // Exact reverse order
}

// ComputeStartupPlan evaluates topological stages using Kahn's algorithm.
func ComputeStartupPlan(g *Graph) (*StartupPlan, error) {
	// 1. Check for cycles first
	cycles := FindCycles(g)
	if len(cycles) > 0 {
		return nil, fmt.Errorf("cannot compute startup plan due to circular dependency: %s", cycles[0].Path)
	}

	// 2. In-degree calculation based on dependencies:
	// A service's dependency count is the number of services in g.Adj[service] that exist in the graph
	inDegree := make(map[string]int)
	for name := range g.Nodes {
		count := 0
		for _, dep := range g.Adj[name] {
			if _, ok := g.Nodes[dep]; ok {
				count++
			}
		}
		inDegree[name] = count
	}

	// 3. BFS wave-by-wave execution
	var stages [][]string
	var flatOrder []string
	visited := make(map[string]bool)

	for len(visited) < len(g.Nodes) {
		var currentStage []string
		for name, deg := range inDegree {
			if deg == 0 && !visited[name] {
				currentStage = append(currentStage, name)
			}
		}

		if len(currentStage) == 0 {
			// Remaining nodes have unresolved in-degrees
			break
		}

		sort.Strings(currentStage)
		stages = append(stages, currentStage)

		for _, name := range currentStage {
			visited[name] = true
			flatOrder = append(flatOrder, name)
			// For each service that depends on this service, decrement its inDegree
			for _, dependent := range g.RevAdj[name] {
				if inDegree[dependent] > 0 {
					inDegree[dependent]--
				}
			}
		}
	}

	// 4. Compute shutdown order
	shutdownOrder := make([]string, len(flatOrder))
	for i, name := range flatOrder {
		shutdownOrder[len(flatOrder)-1-i] = name
	}

	return &StartupPlan{
		Stages:        stages,
		Order:         flatOrder,
		ShutdownOrder: shutdownOrder,
	}, nil
}

// ToMermaid generates a Mermaid flowchart TD representation of the graph.
func (g *Graph) ToMermaid() string {
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")

	// Declare nodes
	var nodeNames []string
	for name := range g.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	for _, name := range nodeNames {
		node := g.Nodes[name]
		label := name
		if node.Image != "" {
			label = fmt.Sprintf("%s<br/><small>%s</small>", name, node.Image)
		}
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", sanitizeID(name), label))
	}

	// Declare edges
	for _, e := range g.Edges {
		label := ""
		if e.Condition == ConditionHealthy {
			label = "|healthy|"
		} else if e.Condition == ConditionCompleted {
			label = "|completed|"
		}
		sb.WriteString(fmt.Sprintf("    %s -->%s %s\n", sanitizeID(e.From), label, sanitizeID(e.To)))
	}

	return sb.String()
}

// ToDOT generates a Graphviz DOT representation of the graph.
func (g *Graph) ToDOT() string {
	var sb strings.Builder
	sb.WriteString("digraph DockerTopology {\n")
	sb.WriteString("    rankdir=LR;\n")
	sb.WriteString("    node [shape=box, style=\"rounded,filled\", fillcolor=\"#1e293b\", fontcolor=\"#f8fafc\", fontname=\"Helvetica\"];\n")
	sb.WriteString("    edge [color=\"#64748b\", fontcolor=\"#94a3b8\", fontname=\"Helvetica\", fontsize=10];\n")

	for _, e := range g.Edges {
		lbl := ""
		if e.Condition != ConditionStarted && e.Condition != "" {
			lbl = fmt.Sprintf(" [label=\"%s\"]", e.Condition)
		}
		sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\"%s;\n", e.From, e.To, lbl))
	}

	sb.WriteString("}\n")
	return sb.String()
}

// ToASCII generates an ASCII tree/list view of startup stages.
func (sp *StartupPlan) ToASCII() string {
	var sb strings.Builder
	sb.WriteString("Docker Compose Topological Startup Order:\n")
	for i, stage := range sp.Stages {
		sb.WriteString(fmt.Sprintf("  Stage %d:\n", i))
		for _, s := range stage {
			sb.WriteString(fmt.Sprintf("    ├── %s\n", s))
		}
	}
	return sb.String()
}

func sanitizeID(id string) string {
	replacer := strings.NewReplacer("-", "_", ".", "_", ":", "_", "/", "_")
	return replacer.Replace(id)
}
