# StackLoom

Interactive Docker Compose topology studio, service dependency graph visualizer, and port conflict linter in Go and React 19 (< 30MB RAM).

---

## Overview

StackLoom solves common startup issues and provides observability in multi-container Docker Compose stacks:

1. **Port collisions**: Prevents `bind: address already in use` failures by testing local host socket availability before starting containers.
2. **Circular dependencies**: Detects cyclic deadlocks in `depends_on` configurations using Tarjan's strongly connected components algorithm.
3. **Missing environment variables**: Scans for `${VARIABLE}` references without defaults or `.env` entries.
4. **Boot race conditions**: Flags services that depend on un-probed databases without proper healthchecks.
5. **Cascading failure simulation**: Calculates the blast radius of an outage on any service, highlighting cascading crash risks across the cluster.
6. **1-click auto-fixer**: Applies suggested port remappings and resource limits directly into `docker-compose.override.yml`.
7. **Selective sub-graph execution**: Lets developers start only dependencies (such as PostgreSQL and Redis) instead of running an entire 15-service stack.
8. **Live hot-reload**: Automatically reloads the graph and linter diagnostics when compose files or `.env` change on disk.

The application compiles into a single standalone binary under 15MB with an embedded React 19 visual canvas. It operates both as an offline static linter and as a live Docker supervisor.

---

## Architecture

```
[ docker-compose.yml / .env / overrides ]
                   │
                   ▼ (Live File Watcher)
┌────────────────────────────────────────────────────────┐
│                   StackLoom Engine                     │
│                                                        │
│  1. Compose Spec v2 Parser & Overrides Merger          │
│  2. Directed Acyclic Graph (DAG) Builder               │
│  3. Tarjan SCC Cycle Detector                          │
│  4. Kahn Topological Startup Wave Sorter               │
│  5. Cascading Blast Radius Calculator                  │
│  6. Pre-Flight Linter & Local TCP Socket Prober        │
│  7. Automated Override Fixer (with Safe Backups)       │
│  8. Smart HTTP Endpoint Prober                         │
│  9. Docker Socket Bridge (/var/run/docker.sock)        │
│                                                        │
│  ┌───────────────────────────┐ ┌────────────────────┐  │
│  │ Embedded React 19 Canvas  │ │ CLI / JSON / HTML  │  │
│  │ (Dagre Auto-Layout)       │ │ PNG / SVG Exporter │  │
│  └───────────────────────────┘ └────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

---

## Installation

### From Source (Go 1.23+)

```bash
git clone https://github.com/alexandrmotologa/stackloom.git
cd stackloom
make build
./stackloom version
```

### Pre-built Binaries

Binaries for Linux, macOS, and Windows are available on the GitHub Releases page.

### Docker Container

```bash
docker run -d \
  -p 9095:9095 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/docker-compose.yml:/workspace/docker-compose.yml \
  ghcr.io/alexandrmotologa/stackloom:latest
```

---

## Usage

### 1. Interactive Topology Studio (Web UI)

Start the web cockpit and open your browser at `http://127.0.0.1:9095`:

```bash
# Default (auto-detects docker-compose.yml)
stackloom serve

# Custom port and compose files
stackloom serve -p 9095 -f docker-compose.yml -f docker-compose.override.yml

# Activate specific profiles
stackloom serve -f docker-compose.yml --profile background,workers
```

Features in the web dashboard:
- Interactive 2D graph with Dagre automatic hierarchical layout.
- Real-time CPU percentage and memory metrics streamed over WebSockets.
- What-If Failure Simulator: click "What-If" on any node to view cascading blast radius.
- Pre-flight linter drawer with 1-click "Apply Fix" button that updates `docker-compose.override.yml`.
- Smart HTTP web link pills: open published web services directly in your browser with live latency checks.
- Docker network filter: highlight services sharing a specific bridge network.
- High-resolution canvas export to PNG, vector SVG, and standalone offline HTML.
- Live file watcher: automatically refreshes the canvas when compose files or `.env` are modified on disk.

### 2. Cascading Failure Simulator (Blast Radius CLI)

Simulate what happens if a specific service experiences an outage:

```bash
stackloom blast-radius -f docker-compose.yml postgres
```

Example output:

```
Outage Simulation: Failure of service "postgres"
Cluster Blast Radius: 4 of 8 services affected (50.0% of cluster)

Cascading Failure Impact Tree:
  ├── Cascade Level 1:
  │    ├── auth-service [FATAL CRASH] (awaits postgres via service_healthy)
  │    ├── billing-service [FATAL CRASH] (awaits postgres via service_healthy)
  ├── Cascade Level 2:
  │    ├── api-gateway [DEGRADED] (awaits auth-service via service_started)
  ├── Cascade Level 3:
  │    ├── frontend [DEGRADED] (awaits api-gateway via service_started)

Unaffected Services:
  • rabbitmq, redis, worker
```

### 3. Pre-Flight Linter & 1-Click Fixer

Run static checks in your terminal:

```bash
# Human-readable terminal output
stackloom lint -f docker-compose.yml

# Machine-readable JSON output for CI automation
stackloom lint -f docker-compose.yml --format json

# Interactive wizard to apply suggested fixes to docker-compose.override.yml
stackloom fix -f docker-compose.yml

# Automatically apply all fixes without prompts
stackloom fix -f docker-compose.yml --auto
```

### 4. Dependency Graph Visualizer

Print the dependency hierarchy directly in the console:

```bash
# ASCII startup stages
stackloom graph -f docker-compose.yml

# Mermaid flowchart syntax
stackloom graph -f docker-compose.yml --format mermaid

# Graphviz DOT syntax
stackloom graph -f docker-compose.yml --format dot
```

### 5. Standalone HTML Export

Generate a self-contained HTML report with the complete interactive graph and linter diagnostics:

```bash
stackloom export -f docker-compose.yml -o topology-report.html
```

---

## Pre-Flight Linter Rules

| Rule ID | Severity | Verification Criteria | Actionable Recommendation |
| :--- | :--- | :--- | :--- |
| `ERR_PORT_COLLISION` | CRITICAL | Published port is already in use locally or duplicated in Compose | Scans for next available port and generates YAML override snippet |
| `ERR_CIRCULAR_DEPENDS` | CRITICAL | Circular loop detected in service dependencies | Identifies exact cycle path using Tarjan's SCC algorithm |
| `ERR_MISSING_DEPENDENCY`| CRITICAL | Service references non-existent service name in `depends_on` | Highlights missing service definition |
| `WARN_UNSET_ENV_VAR` | WARNING | `${VAR}` referenced without default value or `.env` entry | Lists missing variables |
| `WARN_HEALTHCHECK_MISS` | WARNING | Service awaited with `condition: service_healthy` lacks healthcheck | Generates healthcheck block |
| `WARN_NETWORK_ISOLATION`| WARNING | Dependent services share no common Docker networks | Advises adding shared network |
| `WARN_MISSING_BIND_MOUNT`| WARNING | Host file or directory for bind mount does not exist | Warns before Docker creates root-owned directory |
| `WARN_NO_RESOURCE_LIMITS`| WARNING | Service lacks memory and CPU ceilings | Generates `deploy.resources.limits` configuration |
| `SEC_ROOT_USER` | SECURITY | Container runs without non-root user directive | Suggests `user: "1000:1000"` |
| `SEC_PRIVILEGED` | SECURITY | Container runs with `privileged: true` | Warns about host privilege escalation risks |
| `SEC_DOCKER_SOCKET` | SECURITY | Container mounts `/var/run/docker.sock` | Flags container breakout potential |
| `SEC_HOST_NETWORK` | SECURITY | Container uses `network_mode: host` | Warns about network namespace bypass |

---

## Docker Engine Connection

StackLoom automatically checks the following Docker socket endpoints:
- Linux / macOS: `unix:///var/run/docker.sock`
- Windows: `npipe:////./pipe/docker_engine`
- Remote: `tcp://127.0.0.1:2375` or the `DOCKER_HOST` environment variable

When Docker is unavailable or stopped, StackLoom switches to offline mode. All parsing, graph layout, host port probing, and linter capabilities remain active.

---

## License

MIT License. Copyright (c) Alexandr Motologa.
