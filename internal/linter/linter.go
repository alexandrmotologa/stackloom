package linter

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/stackloom/internal/compose"
	"github.com/alexandrmotologa/stackloom/internal/graph"
)

// Severity indicates finding criticality.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeveritySecurity Severity = "SECURITY"
)

// Finding represents an actionable linter discovery.
type Finding struct {
	RuleID            string   `json:"rule_id"`
	Severity          Severity `json:"severity"`
	Service           string   `json:"service,omitempty"`
	Message           string   `json:"message"`
	Detail            string   `json:"detail,omitempty"`
	Recommendation    string   `json:"recommendation"`
	SuggestedOverride string   `json:"suggested_override,omitempty"`
}

// Linter executes pre-flight checks against compose topology.
type Linter struct {
	probeHostSockets bool
}

// NewLinter constructs a new Linter engine.
func NewLinter(probeHostSockets bool) *Linter {
	return &Linter{probeHostSockets: probeHostSockets}
}

// Result groups all findings from a linter run.
type Result struct {
	Findings     []Finding `json:"findings"`
	CriticalCount int      `json:"critical_count"`
	WarningCount  int      `json:"warning_count"`
	SecurityCount int      `json:"security_count"`
	Passed        bool     `json:"passed"`
}

// Lint runs all static and runtime pre-flight rules.
func (l *Linter) Lint(cfg *compose.ComposeConfig, g *graph.Graph, unsetEnvs []string, basePath string) *Result {
	res := &Result{
		Findings: make([]Finding, 0),
		Passed:   true,
	}

	if cfg == nil {
		return res
	}

	// 1. Dependency checks
	l.checkMissingDependencies(cfg, res)
	l.checkCircularDependencies(g, res)
	l.checkHealthCheckDependencies(cfg, res)

	// 2. Port collisions (internal & host socket probe)
	l.checkPortCollisions(cfg, res)

	// 3. Unset environment variables
	l.checkUnsetEnvs(cfg, unsetEnvs, res)

	// 4. Network isolation
	l.checkNetworkIsolation(cfg, res)

	// 5. Missing bind mount files
	l.checkBindMounts(cfg, basePath, res)

	// 6. Resource limits
	l.checkResourceLimits(cfg, res)

	// 7. Security posture
	l.checkSecurity(cfg, res)

	// Count totals
	for _, f := range res.Findings {
		switch f.Severity {
		case SeverityCritical:
			res.CriticalCount++
			res.Passed = false
		case SeverityWarning:
			res.WarningCount++
		case SeveritySecurity:
			res.SecurityCount++
		}
	}

	return res
}

func (l *Linter) checkMissingDependencies(cfg *compose.ComposeConfig, res *Result) {
	for name, s := range cfg.Services {
		for dep := range s.DependsOn {
			if _, exists := cfg.Services[dep]; !exists {
				res.Findings = append(res.Findings, Finding{
					RuleID:         "ERR_MISSING_DEPENDENCY",
					Severity:       SeverityCritical,
					Service:        name,
					Message:        fmt.Sprintf("Service %q depends on non-existent service %q", name, dep),
					Detail:         fmt.Sprintf("declared in services.%s.depends_on", name),
					Recommendation: fmt.Sprintf("Define service %q in the compose file or remove the dependency reference.", dep),
				})
			}
		}
	}
}

func (l *Linter) checkCircularDependencies(g *graph.Graph, res *Result) {
	if g == nil {
		return
	}
	cycles := graph.FindCycles(g)
	for _, c := range cycles {
		res.Findings = append(res.Findings, Finding{
			RuleID:         "ERR_CIRCULAR_DEPENDS",
			Severity:       SeverityCritical,
			Service:        c.Services[0],
			Message:        fmt.Sprintf("Circular dependency deadlock: %s", c.Path),
			Detail:         "Docker Compose cannot determine a deterministic startup order when dependency cycles exist.",
			Recommendation: "Break the cycle by removing one of the depends_on conditions or restructuring initialization.",
		})
	}
}

func (l *Linter) checkHealthCheckDependencies(cfg *compose.ComposeConfig, res *Result) {
	for name, s := range cfg.Services {
		for depName, cond := range s.DependsOn {
			if cond.Condition == string(graph.ConditionHealthy) {
				depSvc, exists := cfg.Services[depName]
				if exists && (depSvc.HealthCheck == nil || depSvc.HealthCheck.Disable) {
					overrideYAML := fmt.Sprintf("services:\n  %s:\n    healthcheck:\n      test: [\"CMD\", \"curl\", \"-f\", \"http://localhost\"]\n      interval: 10s\n      timeout: 5s\n      retries: 3", depName)
					res.Findings = append(res.Findings, Finding{
						RuleID:            "WARN_HEALTHCHECK_MISS",
						Severity:          SeverityWarning,
						Service:           name,
						Message:           fmt.Sprintf("Service %q awaits %q to be healthy, but %q has no healthcheck defined", name, depName, depName),
						Detail:            fmt.Sprintf("Condition 'service_healthy' will block forever or fail if %q lacks a healthcheck probe.", depName),
						Recommendation:    fmt.Sprintf("Add a healthcheck definition to %q or change the condition to 'service_started'.", depName),
						SuggestedOverride: overrideYAML,
					})
				}
			}
		}
	}
}

func (l *Linter) checkPortCollisions(cfg *compose.ComposeConfig, res *Result) {
	type portClaim struct {
		service  string
		protocol string
		target   int
	}
	claimedHostPorts := make(map[int][]portClaim)

	for name, s := range cfg.Services {
		for _, p := range s.Ports {
			if p.Published > 0 {
				proto := p.Protocol
				if proto == "" {
					proto = "tcp"
				}
				claimedHostPorts[p.Published] = append(claimedHostPorts[p.Published], portClaim{
					service:  name,
					protocol: proto,
					target:   p.Target,
				})
			}
		}
	}

	// 1. Internal compose file collision (multiple services targeting same host port)
	for port, claims := range claimedHostPorts {
		if len(claims) > 1 {
			var svcs []string
			for _, c := range claims {
				svcs = append(svcs, c.service)
			}
			sort.Strings(svcs)

			altPort := l.findNextAvailablePort(port + 1)
			overrideYAML := fmt.Sprintf("services:\n  %s:\n    ports:\n      - \"%d:%d\"", claims[1].service, altPort, claims[1].target)

			res.Findings = append(res.Findings, Finding{
				RuleID:            "ERR_PORT_COLLISION",
				Severity:          SeverityCritical,
				Service:           claims[0].service,
				Message:           fmt.Sprintf("Internal port collision on host port %d between services: %s", port, strings.Join(svcs, ", ")),
				Detail:            fmt.Sprintf("Multiple containers attempt to bind to host port %d simultaneously.", port),
				Recommendation:    fmt.Sprintf("Remap one of the services to another host port such as %d.", altPort),
				SuggestedOverride: overrideYAML,
			})
		}
	}

	// 2. Real host socket probing (testing if host port is currently occupied by local processes)
	if l.probeHostSockets {
		for port, claims := range claimedHostPorts {
			if len(claims) == 1 {
				claim := claims[0]
				if claim.protocol == "tcp" {
					if isPortInUse(port) {
						altPort := l.findNextAvailablePort(port + 1)
						overrideYAML := fmt.Sprintf("services:\n  %s:\n    ports:\n      - \"%d:%d\"", claim.service, altPort, claim.target)

						res.Findings = append(res.Findings, Finding{
							RuleID:            "ERR_PORT_COLLISION",
							Severity:          SeverityCritical,
							Service:           claim.service,
							Message:           fmt.Sprintf("Host port %d is already occupied by another running local process", port),
							Detail:            fmt.Sprintf("Starting service %q will immediately fail with 'bind: address already in use: %d'.", claim.service, port),
							Recommendation:    fmt.Sprintf("Stop the process holding port %d, or remap this service to free port %d.", port, altPort),
							SuggestedOverride: overrideYAML,
						})
					}
				}
			}
		}
	}
}

func (l *Linter) findNextAvailablePort(start int) int {
	for p := start; p < start+200; p++ {
		if !isPortInUse(p) {
			return p
		}
	}
	return start + 1000
}

func isPortInUse(port int) bool {
	// Try to listen on the port on 127.0.0.1
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return true
	}
	_ = ln.Close()

	// Also try dial with short timeout to catch wildcard listeners
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true
	}

	return false
}

func (l *Linter) checkUnsetEnvs(cfg *compose.ComposeConfig, unsetEnvs []string, res *Result) {
	if len(unsetEnvs) == 0 {
		return
	}
	sort.Strings(unsetEnvs)
	for _, v := range unsetEnvs {
		res.Findings = append(res.Findings, Finding{
			RuleID:         "WARN_UNSET_ENV_VAR",
			Severity:       SeverityWarning,
			Message:        fmt.Sprintf("Environment variable ${%s} is not defined in host environment or .env file", v),
			Detail:         "Referenced in Compose configuration without fallback value.",
			Recommendation: fmt.Sprintf("Add %s=<value> to your .env file or provide a default like ${%s:-default}.", v, v),
		})
	}
}

func (l *Linter) checkNetworkIsolation(cfg *compose.ComposeConfig, res *Result) {
	for name, s := range cfg.Services {
		// If service is in host network mode, it doesn't need bridge network
		if s.NetworkMode == "host" {
			continue
		}

		sNets := make(map[string]bool)
		for _, n := range s.Networks {
			sNets[n] = true
		}
		// If no networks explicitly defined, compose uses default network
		if len(sNets) == 0 {
			sNets["default"] = true
		}

		for depName := range s.DependsOn {
			depSvc, exists := cfg.Services[depName]
			if !exists || depSvc.NetworkMode == "host" {
				continue
			}

			depNets := make(map[string]bool)
			for _, n := range depSvc.Networks {
				depNets[n] = true
			}
			if len(depNets) == 0 {
				depNets["default"] = true
			}

			// Check intersection
			hasCommon := false
			for n := range sNets {
				if depNets[n] {
					hasCommon = true
					break
				}
			}

			if !hasCommon {
				res.Findings = append(res.Findings, Finding{
					RuleID:         "WARN_NETWORK_ISOLATION",
					Severity:       SeverityWarning,
					Service:        name,
					Message:        fmt.Sprintf("Network isolation: %q and its dependency %q share no common Docker networks", name, depName),
					Detail:         fmt.Sprintf("%q cannot communicate with %q via container DNS names.", name, depName),
					Recommendation: fmt.Sprintf("Add both %q and %q to the same user-defined network.", name, depName),
				})
			}
		}
	}
}

func (l *Linter) checkBindMounts(cfg *compose.ComposeConfig, basePath string, res *Result) {
	if basePath == "" {
		basePath = "."
	}

	for name, s := range cfg.Services {
		for _, v := range s.Volumes {
			if v.Type == "bind" && v.Source != "" {
				// Don't test special devices like /var/run/docker.sock here (handled by security rule)
				if v.Source == "/var/run/docker.sock" || v.Source == "//./pipe/docker_engine" {
					continue
				}

				var checkPath string
				if filepath.IsAbs(v.Source) {
					checkPath = v.Source
				} else {
					checkPath = filepath.Join(basePath, v.Source)
				}

				if _, err := os.Stat(checkPath); os.IsNotExist(err) {
					res.Findings = append(res.Findings, Finding{
						RuleID:         "WARN_MISSING_BIND_MOUNT",
						Severity:       SeverityWarning,
						Service:        name,
						Message:        fmt.Sprintf("Bind mount host path does not exist: %s", v.Source),
						Detail:         fmt.Sprintf("Docker will automatically create an empty directory owned by root at %q if it is missing.", v.Source),
						Recommendation: fmt.Sprintf("Create the file or directory at %q before running docker compose up.", v.Source),
					})
				}
			}
		}
	}
}

func (l *Linter) checkResourceLimits(cfg *compose.ComposeConfig, res *Result) {
	for name, s := range cfg.Services {
		hasLimits := s.Deploy != nil && s.Deploy.Resources != nil && s.Deploy.Resources.Limits != nil &&
			(s.Deploy.Resources.Limits.Memory != "" || s.Deploy.Resources.Limits.CPUs != "")

		if !hasLimits {
			overrideYAML := fmt.Sprintf("services:\n  %s:\n    deploy:\n      resources:\n        limits:\n          cpus: '1.0'\n          memory: 512M", name)
			res.Findings = append(res.Findings, Finding{
				RuleID:            "WARN_NO_RESOURCE_LIMITS",
				Severity:          SeverityWarning,
				Service:           name,
				Message:           fmt.Sprintf("Service %q has no memory or CPU resource ceilings configured", name),
				Detail:            "Unconstrained containers can trigger host OOM-killer during memory spikes, taking down neighboring services.",
				Recommendation:    "Set deploy.resources.limits (memory and cpus) to prevent resource starvation.",
				SuggestedOverride: overrideYAML,
			})
		}
	}
}

func (l *Linter) checkSecurity(cfg *compose.ComposeConfig, res *Result) {
	for name, s := range cfg.Services {
		// 1. Privileged mode
		if s.Privileged {
			res.Findings = append(res.Findings, Finding{
				RuleID:         "SEC_PRIVILEGED",
				Severity:       SeveritySecurity,
				Service:        name,
				Message:        fmt.Sprintf("Service %q runs with privileged: true", name),
				Detail:         "Privileged containers gain full capabilities and access to host devices, bypassing container isolation boundaries.",
				Recommendation: "Drop privileged: true and grant only required specific Linux capabilities via cap_add.",
			})
		}

		// 2. Root user
		if s.User == "" || s.User == "root" || s.User == "0" {
			overrideYAML := fmt.Sprintf("services:\n  %s:\n    user: \"1000:1000\"", name)
			res.Findings = append(res.Findings, Finding{
				RuleID:            "SEC_ROOT_USER",
				Severity:          SeveritySecurity,
				Service:           name,
				Message:           fmt.Sprintf("Service %q runs as root (no non-root user defined)", name),
				Detail:            "Running processes as root inside containers increases the blast radius of remote code execution exploits.",
				Recommendation:    "Explicitly set user: \"1000:1000\" or define a non-root USER directive in the Dockerfile.",
				SuggestedOverride: overrideYAML,
			})
		}

		// 3. Docker socket mount
		for _, v := range s.Volumes {
			if strings.Contains(v.Source, "docker.sock") || strings.Contains(v.Source, "docker_engine") {
				res.Findings = append(res.Findings, Finding{
					RuleID:         "SEC_DOCKER_SOCKET",
					Severity:       SeveritySecurity,
					Service:        name,
					Message:        fmt.Sprintf("Service %q mounts host Docker socket (%s)", name, v.Source),
					Detail:         "Any container with access to docker.sock has effective root access on the host system.",
					Recommendation: "Avoid mounting the Docker socket unless running a trusted container orchestration agent.",
				})
			}
		}

		// 4. Host networking
		if s.NetworkMode == "host" {
			res.Findings = append(res.Findings, Finding{
				RuleID:         "SEC_HOST_NETWORK",
				Severity:       SeveritySecurity,
				Service:        name,
				Message:        fmt.Sprintf("Service %q uses network_mode: host", name),
				Detail:         "Host networking bypasses Docker network isolation and exposes container ports directly on the host interface.",
				Recommendation: "Use standard bridge networks with explicit port mappings instead of host network mode.",
			})
		}
	}
}
