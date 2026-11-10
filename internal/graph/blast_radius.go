package graph

import (
	"sort"
)

// ImpactSeverity indicates the direct consequence of the upstream failure.
type ImpactSeverity string

const (
	SeverityFatal    ImpactSeverity = "FATAL"    // Service requires healthy upstream; will block or crash
	SeverityDegraded ImpactSeverity = "DEGRADED" // Service started after upstream, but loses runtime communication
)

// ImpactNode details one service impacted by an outage.
type ImpactNode struct {
	Service     string         `json:"service"`
	Depth       int            `json:"depth"`        // 1 = direct dependent, 2 = 2nd level dependent, etc.
	DirectCause string         `json:"direct_cause"` // Upstream service triggering this failure
	Condition   string         `json:"condition"`    // "service_healthy", "service_started"
	Severity    ImpactSeverity `json:"severity"`     // "FATAL" or "DEGRADED"
}

// BlastRadiusReport summarizes the blast radius of an outage on a specific service.
type BlastRadiusReport struct {
	TargetService    string       `json:"target_service"`
	TotalServices    int          `json:"total_services"`
	AffectedCount    int          `json:"affected_count"`
	BlastRadiusRatio float64      `json:"blast_radius_ratio"` // Ratio from 0.0 to 1.0
	ImpactTree       []ImpactNode `json:"impact_tree"`        // Ordered by depth and name
	Unaffected       []string     `json:"unaffected"`
}

// CalculateBlastRadius simulates what happens if targetService experiences an outage.
func (g *Graph) CalculateBlastRadius(targetService string) *BlastRadiusReport {
	report := &BlastRadiusReport{
		TargetService: targetService,
		TotalServices: len(g.Nodes),
		ImpactTree:    make([]ImpactNode, 0),
		Unaffected:    make([]string, 0),
	}

	if _, exists := g.Nodes[targetService]; !exists {
		return report
	}

	// BFS traversal along reverse adjacency (RevAdj): who depends on targetService?
	visited := make(map[string]int) // service -> depth
	visited[targetService] = 0

	type queueItem struct {
		service     string
		depth       int
		directCause string
	}

	var queue []queueItem

	// Initialize with direct dependents
	for _, dep := range g.RevAdj[targetService] {
		queue = append(queue, queueItem{
			service:     dep,
			depth:       1,
			directCause: targetService,
		})
	}

	// Helper to lookup edge condition
	getCondition := func(from, to string) DependencyCondition {
		for _, e := range g.Edges {
			if e.From == from && e.To == to {
				return e.Condition
			}
		}
		return ConditionStarted
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if _, seen := visited[curr.service]; seen {
			continue
		}
		visited[curr.service] = curr.depth

		cond := getCondition(curr.service, curr.directCause)
		severity := SeverityDegraded
		if cond == ConditionHealthy {
			severity = SeverityFatal
		}

		report.ImpactTree = append(report.ImpactTree, ImpactNode{
			Service:     curr.service,
			Depth:       curr.depth,
			DirectCause: curr.directCause,
			Condition:   string(cond),
			Severity:    severity,
		})

		// Enqueue downstream dependents of this service
		for _, next := range g.RevAdj[curr.service] {
			if _, seen := visited[next]; !seen {
				queue = append(queue, queueItem{
					service:     next,
					depth:       curr.depth + 1,
					directCause: curr.service,
				})
			}
		}
	}

	// Sort impact tree by depth, then service name
	sort.Slice(report.ImpactTree, func(i, j int) bool {
		if report.ImpactTree[i].Depth != report.ImpactTree[j].Depth {
			return report.ImpactTree[i].Depth < report.ImpactTree[j].Depth
		}
		return report.ImpactTree[i].Service < report.ImpactTree[j].Service
	})

	report.AffectedCount = len(report.ImpactTree)
	if report.TotalServices > 0 {
		report.BlastRadiusRatio = float64(report.AffectedCount) / float64(report.TotalServices)
	}

	// Compute unaffected nodes
	for name := range g.Nodes {
		if name != targetService {
			if _, affected := visited[name]; !affected {
				report.Unaffected = append(report.Unaffected, name)
			}
		}
	}
	sort.Strings(report.Unaffected)

	return report
}
