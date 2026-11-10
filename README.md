# StackLoom

Interactive Docker Compose topology studio, service dependency graph visualizer, and port conflict linter in Go and React 19 (< 30MB RAM).

---

## Overview

StackLoom solves common startup issues in multi-container Docker Compose stacks:

1. **Port collisions**: Prevents `bind: address already in use` failures by testing local host socket availability before starting containers.
2. **Circular dependencies**: Detects cyclic deadlocks in `depends_on` configurations using Tarjan's strongly connected components algorithm.
3. **Missing environment variables**: Scans for `${VARIABLE}` references without defaults or `.env` entries.
4. **Boot race conditions**: Flags services that depend on un-probed databases without proper healthchecks.
5. **Selective sub-graph execution**: Lets developers start only dependencies (such as PostgreSQL and Redis) instead of running an entire 15-service stack.

The application compiles into a single standalone binary under 15MB with an embedded React 19 visual canvas. It operates both as an offline static linter and as a live Docker supervisor.

---

## Architecture

```
[ docker-compose.yml / .env / overrides ]
                   │
                   ▼
┌────────────────────────────────────────────────────────┐
│                   StackLoom Engine                     │
│                                                        │
│  1. Compose Spec v2 Parser & Overrides Merger          │
│  2. Directed Acyclic Graph (DAG) Builder               │
│  3. Tarjan SCC Cycle Detector                          │
│  4. Kahn Topological Startup Wave Sorter               │
│  5. Pre-Flight Linter & Local TCP Socket Prober        │
│  6. Docker Socket Bridge (/var/run/docker.sock)        │
│                                                        │
│  ┌───────────────────────────┐ ┌────────────────────┐  │
│  │ Embedded React 19 Canvas  │ │ CLI / JSON / HTML  │  │
│  │ (Dagre Auto-Layout)       │ │ Exporter           │  │
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

Features available in the web dashboard:
- Interactive 2D graph with Dagre automatic hierarchical layout.
- Service cards showing image, published ports, volumes, and health status.
- Real-time CPU percentage and memory metrics streamed over WebSockets.
- Pre-flight linter drawer with categorized findings and suggested YAML overrides.
- Live container logs modal with regex search and stdout/stderr stream filters.
- Sub-graph execution: launch databases and cache only with one click.

### 2. Pre-Flight Linter CLI

Run static checks in your terminal or in CI/CD pipelines:

```bash
# Human-readable terminal output
stackloom lint -f docker-compose.yml

# Machine-readable JSON output for CI automation
stackloom lint -f docker-compose.yml --format json

# Skip local host socket probing in remote build environments
stackloom lint -f docker-compose.yml --no-probe
```

Exit code is `1` if critical errors exist, and `0` when all checks pass.

### 3. Dependency Graph Visualizer

Print the dependency hierarchy directly in the console:

```bash
# ASCII startup stages
stackloom graph -f docker-compose.yml

# Mermaid flowchart syntax
stackloom graph -f docker-compose.yml --format mermaid

# Graphviz DOT syntax
stackloom graph -f docker-compose.yml --format dot
```

### 4. Standalone HTML Export

Generate a self-contained HTML report with the complete interactive graph and linter diagnostics:

```bash
stackloom export -f docker-compose.yml -o topology-report.html
```

The exported file runs offline without external scripts or running servers.

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
