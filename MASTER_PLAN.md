# StackLoom: Architecture and Engineering Master Plan

> Interactive Docker Compose topology studio, service dependency graph visualizer, and port conflict linter in Go and React 19 (< 30MB RAM).

---

## 1. Executive Summary & Problem Space

### The Core Problem
Modern engineering repositories rely on multi-service `docker-compose.yml` stacks (frontend, backend, background workers, PostgreSQL, Redis, RabbitMQ, MinIO, Mailpit). As stacks grow to 8 to 20 services, developers face chronic operational friction:
1. **Blind Startup Failures**: Running `docker compose up` fails halfway through because of port collisions (`bind: address already in use: 5432`), circular `depends_on` deadlocks, or uninitialized environment variables.
2. **Container Boot Race Conditions**: Services start concurrently before their backing database is ready to accept connections (healthchecks improperly configured), triggering cascading container crashes and restarts.
3. **Resource Starvation**: Developers run the entire monolithic compose stack when they only need 2 backing services (for instance, PostgreSQL and Redis), exhausting laptop RAM and CPU.

### The Solution: StackLoom
StackLoom is an interactive Docker Compose topology visualizer, pre-flight linter, and runtime supervisor written in Go with an embedded React 19 visual canvas. It parses single and multi-file compose configurations (`docker-compose.override.yml`, profiles), detects port collisions and dependency loops before containers launch, and connects to the local Docker socket to render a live, animated 2D service dependency graph with real-time CPU/memory sparklines and per-container logs.

---

## 2. System Architecture & Topology Pipeline

```
[ Developer / Monorepo (docker-compose.yml, .env, docker-compose.override.yml) ]
                               │
                               │  Parses YAML & Environment
                               ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        StackLoom Daemon                                │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Compose Specification v2 Parser & Merging Engine                 │  │
│  │ - Merges overrides, profiles, variable substitutions             │  │
│  │ - Builds Directed Acyclic Graph (DAG) of services and networks   │  │
│  └──────────────────────────────────┬───────────────────────────────┘  │
│                                     │                                  │
│                                     ▼                                  │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Pre-Flight Static Linter & Port Probe                            │  │
│  │ - Port Collision Detector: tests local TCP socket bindings       │  │
│  │ - Circular depends_on cycle detector (Tarjan's algorithm)        │  │
│  │ - Missing environment variable analyzer (${VAR} -> unset)        │  │
│  │ - Network isolation & missing bind-mount auditor                 │  │
│  │ - Security Advisor: root user, privileged mode, socket mounts    │  │
│  └──────────────────────────────────┬───────────────────────────────┘  │
│                                     │ Verified Graph & Linter Verdicts │
│                                     ▼                                  │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Live Docker Socket Bridge (/var/run/docker.sock or Named Pipe)   │  │
│  │ - Auto-detects local Docker socket / falls back to offline mode  │  │
│  │ - Real-time container state: healthchecks, exit codes, uptime    │  │
│  │ - Streaming CPU % & RAM usage sparklines per node                │  │
│  │ - Demultiplexed container logs streaming                         │  │
│  └──────────────────────────────────┬───────────────────────────────┘  │
│                                     │                                  │
│         ┌───────────────────────────┴──────────────────────────┐       │
│         ▼                                                      ▼       │
│  ┌───────────────────────────────┐           ┌──────────────────────┐  │
│  │ Selective Sub-Graph Runner    │           │ Embedded React 19    │  │
│  │ - "Start DB & Cache Only"     │           │ Topology Canvas      │  │
│  │ - 1-Click service restart     │           │ (Interactive 2D DAG) │  │
│  │ - Standalone HTML exporter    │           │ (Dagre Auto-Layout)  │  │
│  └───────────────────────────────┘           └──────────────────────┘  │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Technology Stack

| Layer | Component | Specification |
| :--- | :--- | :--- |
| **Language & Runtime** | Go 1.23+ | Ultra-fast native execution, low memory footprint (< 30MB RAM), cross-platform |
| **Compose Parser** | Custom Compose Spec v2 Engine | Canonical YAML parser supporting services, networks, volumes, and profiles |
| **Docker Integration** | Docker Engine API Client | Direct communication via Unix domain socket, Windows named pipe, or TCP |
| **Graph Algorithms** | Tarjan's SCC & Topological Sort | Cycle detection in dependency trees and deterministic startup ordering |
| **Embedded Web UI** | React 19 + TypeScript + Vite | Built as a single bundle and served via Go `//go:embed` |
| **Canvas & Layout** | React Flow / Dagre layout | Animated edge connectors, interactive zooming, panning, and minimap |
| **UI Styling** | Tailwind CSS + Lucide Icons | Clean modern dark-mode infrastructure cockpit |

---

## 4. Repository & Directory Structure

```
B:\workgit\stackloom\
├── cmd\
│   └── stackloom\
│       └── main.go                 # Daemon entrypoint, CLI commands (serve, lint, graph, export)
├── internal\
│   ├── config\
│   │   └── config.go               # Configuration (port, compose file paths, socket path)
│   ├── compose\
│   │   ├── parser.go               # YAML parser with environment variable interpolation
│   │   ├── merger.go               # Merges multiple compose files (base + overrides)
│   │   └── models.go               # Service, network, volume, and secret data structures
│   ├── graph\
│   │   ├── dag.go                  # Directed acyclic graph builder
│   │   ├── cycles.go               # Tarjan's cycle detection algorithm
│   │   └── order.go                # Topological startup and shutdown order evaluator
│   ├── linter\
│   │   ├── linter.go               # Linter runner, rule registry, and diagnostic models
│   │   ├── ports.go                # Checks for port collisions against local TCP sockets
│   │   ├── env.go                  # Flags unset or empty environment variables
│   │   ├── networks.go             # Detects network isolation between dependent services
│   │   ├── mounts.go               # Validates local bind mount paths on disk
│   │   └── security.go             # Flags privileged containers, root users, and socket mounts
│   ├── docker\
│   │   ├── client.go               # Docker Engine socket client with auto-discovery
│   │   ├── stats.go                # Live streaming CPU and memory usage
│   │   └── logs.go                 # Demultiplexed container logs streaming
│   ├── export\
│   │   └── html.go                 # Standalone HTML interactive report generator
│   └── api\
│       ├── router.go               # REST & WebSocket API router
│       ├── topology.go             # Graph JSON representation endpoint
│       ├── linter.go               # Linter results and port suggestions endpoint
│       ├── logs.go                 # Streaming WebSocket logs handler
│       ├── stats.go                # Streaming WebSocket stats handler
│       └── actions.go              # Start/Stop/Restart services and sub-graphs
├── web\
│   ├── src\
│   │   ├── components\
│   │   │   ├── TopologyCanvas.tsx  # Interactive 2D DAG with animated service nodes
│   │   │   ├── ServiceNode.tsx     # Custom node: status badge, ports, CPU/RAM sparklines
│   │   │   ├── LinterDrawer.tsx    # Slide-over validation panel with warning badges
│   │   │   ├── LogViewerModal.tsx  # Live streaming logs for selected container node
│   │   │   └── SubGraphControls.tsx# 1-Click "Start DB Only" / "Teardown All" toolbar
│   │   ├── App.tsx                 # Root topology dashboard
│   │   ├── main.tsx                # React 19 entrypoint
│   │   └── index.css               # Tailwind CSS declarations
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── testdata\                       # Sample compose stacks for tests and benchmarks
│   ├── microservices\
│   ├── circular_error\
│   └── overrides\
├── Makefile                        # Multi-platform build and release targets
├── Dockerfile                      # Scratch container image (< 20MB)
├── README.md                       # Comprehensive guide and CLI usage manual
└── MASTER_PLAN.md                  # This master specification
```

---

## 5. Pre-Flight Linter Rules & Verification Criteria

| Linter Rule ID | Severity | Verification Criteria | Actionable Recommendation |
| :--- | :--- | :--- | :--- |
| `ERR_PORT_COLLISION` | **CRITICAL** | Host port is already in use by another local process | Identifies conflicting port, scans for next free port, offers YAML override |
| `ERR_CIRCULAR_DEPENDS`| **CRITICAL** | Service A depends on B, which depends on A | Flags the exact dependency cycle loop using Tarjan's algorithm |
| `ERR_MISSING_DEPENDENCY`| **CRITICAL** | Service depends on non-existent service name | Highlights invalid reference in `depends_on` |
| `WARN_UNSET_ENV_VAR` | **WARNING** | Variable `${KEY}` referenced without default or `.env` value | Lists missing environment variables |
| `WARN_HEALTHCHECK_MISS`| **WARNING** | Service has `condition: service_healthy` dependency without healthcheck | Recommends `healthcheck` definition |
| `WARN_NETWORK_ISOLATION`| **WARNING** | Dependent services share zero common networks | Recommends adding services to shared network |
| `WARN_MISSING_BIND_MOUNT`| **WARNING** | Local bind mount host path does not exist | Warns before Docker creates root-owned directory |
| `WARN_NO_RESOURCE_LIMITS`| **WARNING** | Service lacks CPU or memory limits | Suggests adding `deploy.resources.limits` |
| `SEC_ROOT_USER` | **SECURITY** | Container runs as root without non-root `user: ...` | Suggests `user: "1000:1000"` |
| `SEC_PRIVILEGED` | **SECURITY** | Container sets `privileged: true` | Warns about host privilege escalation risks |
| `SEC_DOCKER_SOCKET` | **SECURITY** | Container mounts host Docker socket | Flags risk of container breakout to host |
| `SEC_HOST_NETWORK` | **SECURITY** | Container uses `network_mode: host` | Warns about network namespace bypass |

---

## 6. Implementation Roadmap

### Phase 1: Compose Parser, Merger & DAG Graph Construction
- Initialize Go module `github.com/alexandrmotologa/stackloom`.
- Implement `compose/models.go` with complete Compose Spec v2 definitions.
- Implement `compose/parser.go` with YAML unmarshaling and variable substitution (`${VAR}`, `${VAR:-default}`, etc.).
- Implement `compose/merger.go` merging base and override files, handling profiles and network merges.
- Implement `graph/dag.go` building directed acyclic graph with service nodes and edges.
- Implement `graph/cycles.go` using Tarjan's strongly connected components algorithm.
- Implement `graph/order.go` evaluating topological startup and shutdown sequences.
- Add comprehensive test suites for parser, merger, and graph logic with real compose files.

### Phase 2: Pre-Flight Linter & Socket Port Prober
- Implement `linter/linter.go` rule engine with categorized severity levels.
- Implement `linter/ports.go` probing local TCP sockets (`net.Listen`) and recommending free replacement ports.
- Implement `linter/env.go` checking unresolved variable interpolations against `.env` and host env.
- Implement `linter/networks.go` validating network adjacency for dependent services.
- Implement `linter/mounts.go` checking local bind mount path existence.
- Implement `linter/security.go` auditing root users, privileged mode, Docker socket mounts, and host networking.
- Build CLI commands:
  - `stackloom lint [files...]` with text and JSON outputs.
  - `stackloom graph [files...]` with ASCII, Mermaid, and Graphviz DOT formats.
  - `stackloom export [files...]` generating standalone interactive HTML report.

### Phase 3: Docker Engine Socket Bridge & Sub-Graph Runner
- Implement `docker/client.go` with auto-discovery (Unix socket, Windows named pipe, TCP).
- Handle graceful fallback to offline studio mode when Docker daemon is not active.
- Implement `docker/stats.go` streaming container CPU percentage and memory metrics.
- Implement `docker/logs.go` streaming demultiplexed stdout/stderr log frames.
- Implement `docker/actions.go` controlling services and sub-graphs (start dependencies only, restart, stop).
- Implement sub-graph dependency resolver (`CalculateSubGraph(services []string, direction)`) for focused execution.

### Phase 4: Embedded React 19 Interactive Topology Canvas
- Scaffold Vite + React 19 + TypeScript frontend in `web/`.
- Configure Tailwind CSS with modern dark infrastructure theme.
- Build interactive 2D DAG canvas using React Flow and Dagre hierarchical layout.
- Build custom ServiceNode component with status badges, port indicators, and real-time CPU/RAM meters.
- Build slide-over Pre-Flight Linter Drawer with categorized diagnostic cards and suggested fixes.
- Build live container Log Viewer Modal with search and stdout/stderr stream filtering.
- Build SubGraphControls toolbar for profile toggling, sub-graph execution, and graph export.
- Wire WebSockets and REST endpoints in `api/` package.
- Embed production web bundle into Go binary using `//go:embed`.

### Phase 5: Production Hardening, Verification & Documentation
- Test with complex enterprise Compose stacks (10+ services, multiple networks, healthchecks, profiles).
- Benchmark memory footprint ensuring resident set size remains < 30MB RAM.
- Write humanized documentation, README.md, usage examples, and architecture overview.
- Build multi-platform release binaries and verify standalone execution.
