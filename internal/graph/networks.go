package graph

import (
	"sort"

	"github.com/alexandrmotologa/stackloom/internal/compose"
)

// NetworkTopology models network connectivity and internal container DNS visibility.
type NetworkTopology struct {
	Networks         map[string][]string `json:"networks"`          // network -> services
	VisibilityMatrix map[string][]string `json:"visibility_matrix"` // service -> reachable services
	IsolatedServices []string            `json:"isolated_services"` // services with 0 connectivity
}

// ComputeNetworkTopology evaluates which services can communicate with each other over Docker networks.
func ComputeNetworkTopology(cfg *compose.ComposeConfig) *NetworkTopology {
	topo := &NetworkTopology{
		Networks:         make(map[string][]string),
		VisibilityMatrix: make(map[string][]string),
		IsolatedServices: make([]string, 0),
	}

	if cfg == nil {
		return topo
	}

	// 1. Group services by network
	serviceNetworks := make(map[string][]string)
	for name, s := range cfg.Services {
		nets := s.Networks
		if len(nets) == 0 {
			nets = []string{"default"}
		}
		serviceNetworks[name] = nets

		for _, n := range nets {
			topo.Networks[n] = append(topo.Networks[n], name)
		}
	}

	// Sort network service lists
	for n := range topo.Networks {
		sort.Strings(topo.Networks[n])
	}

	// 2. Build visibility matrix (services that share at least 1 common network)
	for s1, nets1 := range serviceNetworks {
		reachableMap := make(map[string]bool)
		for s2, nets2 := range serviceNetworks {
			if s1 == s2 {
				continue
			}
			// Check if s1 and s2 share any network
			for _, n1 := range nets1 {
				for _, n2 := range nets2 {
					if n1 == n2 {
						reachableMap[s2] = true
						break
					}
				}
			}
		}

		var reachableList []string
		for r := range reachableMap {
			reachableList = append(reachableList, r)
		}
		sort.Strings(reachableList)
		topo.VisibilityMatrix[s1] = reachableList

		if len(reachableList) == 0 && len(cfg.Services) > 1 {
			topo.IsolatedServices = append(topo.IsolatedServices, s1)
		}
	}

	sort.Strings(topo.IsolatedServices)
	return topo
}
