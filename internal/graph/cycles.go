package graph

import (
	"sort"
)

// Cycle represents a detected circular dependency loop in the topology.
type Cycle struct {
	Services []string `json:"services"` // Closed loop: [A, B, C, A]
	Path     string   `json:"path"`     // "A -> B -> C -> A"
}

// CycleDetector implements Tarjan's Strongly Connected Components algorithm.
type CycleDetector struct {
	g        *Graph
	index    int
	indices  map[string]int
	lowlink  map[string]int
	onStack  map[string]bool
	stack    []string
	cycles   []Cycle
	sccs     [][]string
}

// FindCycles analyzes the graph for circular dependencies.
func FindCycles(g *Graph) []Cycle {
	cd := &CycleDetector{
		g:       g,
		index:   0,
		indices: make(map[string]int),
		lowlink: make(map[string]int),
		onStack: make(map[string]bool),
		stack:   make([]string, 0),
		cycles:  make([]Cycle, 0),
		sccs:    make([][]string, 0),
	}

	// Sort node keys for deterministic analysis
	var nodeNames []string
	for name := range g.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	for _, u := range nodeNames {
		if _, visited := cd.indices[u]; !visited {
			cd.strongConnect(u)
		}
	}

	return cd.cycles
}

func (cd *CycleDetector) strongConnect(u string) {
	cd.indices[u] = cd.index
	cd.lowlink[u] = cd.index
	cd.index++
	cd.stack = append(cd.stack, u)
	cd.onStack[u] = true

	// Explore dependencies (u -> v)
	for _, v := range cd.g.Adj[u] {
		// Only consider nodes that exist in the graph
		if _, exists := cd.g.Nodes[v]; !exists {
			continue
		}

		if _, visited := cd.indices[v]; !visited {
			// Successor v has not yet been visited; recurse
			cd.strongConnect(v)
			if cd.lowlink[v] < cd.lowlink[u] {
				cd.lowlink[u] = cd.lowlink[v]
			}
		} else if cd.onStack[v] {
			// Successor v is on stack and hence in the current SCC
			if cd.indices[v] < cd.lowlink[u] {
				cd.lowlink[u] = cd.indices[v]
			}
		}
	}

	// If u is a root node, pop the stack and generate an SCC
	if cd.lowlink[u] == cd.indices[u] {
		var scc []string
		for {
			w := cd.stack[len(cd.stack)-1]
			cd.stack = cd.stack[:len(cd.stack)-1]
			cd.onStack[w] = false
			scc = append(scc, w)
			if w == u {
				break
			}
		}

		// SCC has a cycle if len > 1 or single node depends on itself
		if len(scc) > 1 {
			cd.extractCycleFromSCC(scc)
		} else if len(scc) == 1 {
			// Check for self loop
			node := scc[0]
			for _, dep := range cd.g.Adj[node] {
				if dep == node {
					cd.cycles = append(cd.cycles, Cycle{
						Services: []string{node, node},
						Path:     node + " -> " + node,
					})
					break
				}
			}
		}
	}
}

// extractCycleFromSCC finds an ordered cycle path within a strongly connected component.
func (cd *CycleDetector) extractCycleFromSCC(scc []string) {
	sccSet := make(map[string]bool)
	for _, s := range scc {
		sccSet[s] = true
	}

	start := scc[0]
	path := []string{start}
	visitedInPath := make(map[string]int)
	visitedInPath[start] = 0

	curr := start
	for {
		foundNext := false
		for _, next := range cd.g.Adj[curr] {
			if sccSet[next] {
				if idx, inPath := visitedInPath[next]; inPath {
					// Closed cycle found from idx to end
					cycleSlice := append([]string{}, path[idx:]...)
					cycleSlice = append(cycleSlice, next)
					cd.cycles = append(cd.cycles, Cycle{
						Services: cycleSlice,
						Path:     formatPath(cycleSlice),
					})
					return
				}
				// Append next and proceed
				path = append(path, next)
				visitedInPath[next] = len(path) - 1
				curr = next
				foundNext = true
				break
			}
		}
		if !foundNext {
			break
		}
	}

	// Fallback if full cycle trace wasn't cleanly closed
	sccCopy := append([]string{}, scc...)
	sccCopy = append(sccCopy, scc[0])
	cd.cycles = append(cd.cycles, Cycle{
		Services: sccCopy,
		Path:     formatPath(sccCopy),
	})
}

func formatPath(nodes []string) string {
	res := ""
	for i, n := range nodes {
		if i > 0 {
			res += " -> "
		}
		res += n
	}
	return res
}
