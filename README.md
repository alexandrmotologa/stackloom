<p align="center">
  <img src="docs/images/stackloom_logo.png?raw=true" alt="StackLoom Logo" width="130" style="border-radius: 24px;" />
</p>

<h1 align="center">StackLoom</h1>

<p align="center">
  <strong>Interactive Docker Compose Topology Studio, Service Dependency Graph & Port Conflict Linter</strong>
</p>

<p align="center">
  <a href="https://github.com/alexandrmotologa/stackloom/actions"><img src="https://img.shields.io/badge/build-passing-brightgreen.svg" alt="Build Status" /></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.23+-00ADD8.svg?logo=go" alt="Go Version" /></a>
  <a href="https://react.dev"><img src="https://img.shields.io/badge/React-19-61DAFB.svg?logo=react" alt="React 19" /></a>
  <a href="https://github.com/alexandrmotologa/stackloom/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT" /></a>
  <img src="https://img.shields.io/badge/RAM-%3C%2030MB-emerald.svg" alt="Memory footprint" />
</p>

<p align="center">
  <img src="docs/images/stackloom_demo.gif?raw=true" alt="StackLoom Interactive Demo" width="100%" />
</p>

---

## Overview

StackLoom inspects multi-container Docker Compose stacks to solve startup errors and provide real-time architectural visibility:

1. **Port collisions**: Prevents `bind: address already in use` crashes by verifying host TCP socket availability before containers start.
2. **Circular dependencies**: Detects cyclic deadlocks in `depends_on` configurations using Tarjan's strongly connected components algorithm.
3. **Missing environment variables**: Scans for `${VARIABLE}` expressions without defaults or `.env` file entries.
4. **Boot race conditions**: Flags services that depend on un-probed databases without healthcheck declarations.
5. **Cascading failure simulation**: Computes the blast radius of a simulated service outage, highlighting downstream crash risks across the cluster.
6. **1-click auto-fixer**: Writes suggested port remappings and resource limits directly into `docker-compose.override.yml`.
7. **Selective sub-graph execution**: Starts isolated dependencies (such as PostgreSQL and Redis) instead of booting the entire application cluster.
8. **Live hot-reload**: Refreshes graph topology and diagnostics whenever compose files or `.env` change on disk.

The system compiles into a single binary under 15MB with an embedded React 19 visual canvas. It functions as an offline static linter and as a live Docker supervisor consuming less than 30MB RAM.

---

## Visual Studio Showcase

| Topology Studio Dashboard | Cascading Blast Radius Outage Simulator |
| :---: | :---: |
| ![StackLoom Dashboard](docs/images/stackloom_dashboard.png?raw=true) | ![StackLoom Blast Radius](docs/images/stackloom_blast_radius.png?raw=true) |
| *Interactive DAG topology with live healthcheck links and startup waves* | *Outage simulation with crash risk vectors across dependency tiers* |

| Pre-Flight Linter & 1-Click Fixer | Responsive Mobile Cockpit |
| :---: | :---: |
| ![StackLoom Linter](docs/images/stackloom_linter.png?raw=true) | ![StackLoom Mobile](docs/images/stackloom_mobile.png?raw=true) |
| *Static socket collisions and docker-compose.override.yml generator* | *Adaptive layout supporting phone and tablet viewports* |

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

Start the web studio and open `http://127.0.0.1:9095`:

```bash
# Auto-detects docker-compose.yml in current directory
stackloom serve

# Custom port and multiple compose files
stackloom serve -p 9095 -f docker-compose.yml -f docker-compose.override.yml

# Activate specific compose profiles
stackloom serve -f docker-compose.yml --profile background,workers
```

Features in the visual interface:
- Interactive 2D graph with Dagre automatic hierarchical layout.
- Real-time CPU percentage and memory metrics streamed over WebSockets.
- What-If Failure Simulator: test how service downtime propagates to downstream consumers.
- Pre-flight linter drawer with a 1-click "Apply Fix" button modifying `docker-compose.override.yml`.
- Smart HTTP web link pills: open published web services directly in your browser with live latency checks.
- Docker network filter: highlight services sharing a specific bridge network.
- High-resolution canvas export to PNG, vector SVG, and standalone offline HTML.
- Live file watcher: automatically reloads the canvas when compose files or `.env` are saved on disk.

### 2. Cascading Failure Simulator (Blast Radius CLI)

Simulate outages from the command line:

```bash
stackloom blast-radius -f docker-compose.yml postgres
```

Example terminal report:

```
Outage Simulation: Failure of service "postgres"
Cluster Blast Radius: 4 of 7 services affected (57.1% of cluster)

Cascading Failure Impact Tree:
  ├── Cascade Level 1:
  │    ├── auth-service [FATAL CRASH] (awaits postgres via service_healthy)
  │    ├── billing-service [FATAL CRASH] (awaits postgres via service_healthy)
  ├── Cascade Level 2:
  │    ├── api-gateway [DEGRADED] (awaits auth-service via service_started)
  ├── Cascade Level 3:
  │    ├── frontend [DEGRADED] (awaits api-gateway via service_started)

Unaffected Services:
  • rabbitmq, redis
```

### 3. Pre-Flight Linter & 1-Click Fixer

Run static diagnostics in your terminal:

```bash
# Terminal output with color badges
stackloom lint -f docker-compose.yml

# Machine-readable JSON output for CI pipelines
stackloom lint -f docker-compose.yml --format json

# Interactive wizard to generate docker-compose.override.yml
stackloom fix -f docker-compose.yml

# Automatically apply recommended fixes without manual prompts
stackloom fix -f docker-compose.yml --auto
```

### 4. Dependency Graph Visualizer

Print dependency hierarchies directly to standard output:

```bash
# Topological startup wave summary
stackloom graph -f docker-compose.yml

# Mermaid flowchart syntax
stackloom graph -f docker-compose.yml --format mermaid

# Graphviz DOT syntax
stackloom graph -f docker-compose.yml --format dot
```

### 5. Standalone HTML Export

Generate a self-contained offline report with interactive graph and diagnostics:

```bash
stackloom export -f docker-compose.yml -o topology-report.html
```

---

## Pre-Flight Linter Rules

| Rule ID | Severity | Verification Criteria | Actionable Recommendation |
| :--- | :--- | :--- | :--- |
| `ERR_PORT_COLLISION` | CRITICAL | Published port is occupied on host or duplicated across containers | Scans for next available host port and outputs override snippet |
| `ERR_CIRCULAR_DEPENDS` | CRITICAL | Circular loop detected in service dependency chain | Identifies the exact cycle path using Tarjan's SCC algorithm |
| `ERR_MISSING_DEPENDENCY`| CRITICAL | Service references an undefined service in `depends_on` | Points out missing service definition |
| `WARN_UNSET_ENV_VAR` | WARNING | `${VAR}` referenced without default value or `.env` entry | Lists missing variables requiring definition |
| `WARN_HEALTHCHECK_MISS` | WARNING | Service awaited with `condition: service_healthy` lacks a healthcheck | Generates standard healthcheck test block |
| `WARN_NETWORK_ISOLATION`| WARNING | Dependent services do not share a common Docker network | Advises adding a shared network attachment |
| `WARN_MISSING_BIND_MOUNT`| WARNING | Host file or directory for bind mount does not exist on disk | Warns before Docker creates an unexpected root-owned directory |
| `WARN_NO_RESOURCE_LIMITS`| WARNING | Service lacks memory and CPU resource caps | Generates `deploy.resources.limits` configuration |
| `SEC_ROOT_USER` | SECURITY | Container runs without non-root user directive | Suggests non-root `user: "1000:1000"` |
| `SEC_PRIVILEGED` | SECURITY | Container runs with `privileged: true` | Warns about host privilege escalation risks |
| `SEC_DOCKER_SOCKET` | SECURITY | Container mounts `/var/run/docker.sock` | Flags container breakout potential |
| `SEC_HOST_NETWORK` | SECURITY | Container uses `network_mode: host` | Warns about network namespace bypass |

---

## Docker Engine Connection

StackLoom automatically connects to local Docker socket endpoints:
- Linux / macOS: `unix:///var/run/docker.sock`
- Windows: `npipe:////./pipe/docker_engine`
- Remote: `tcp://127.0.0.1:2375` or the `DOCKER_HOST` environment variable

When Docker is unavailable or stopped, StackLoom switches to offline mode. All parsing, graph layout, host port probing, and linter capabilities remain active.

---

## License

MIT License. Copyright (c) Alexandr Motologa.
