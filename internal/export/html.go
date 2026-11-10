package export

import (
	"encoding/json"
	"fmt"
	"html"

	"github.com/alexandrmotologa/stackloom/internal/compose"
	"github.com/alexandrmotologa/stackloom/internal/graph"
	"github.com/alexandrmotologa/stackloom/internal/linter"
)

// ReportData holds the complete payload bundled into the standalone HTML report.
type ReportData struct {
	Compose     *compose.ComposeConfig `json:"compose"`
	Graph       *graph.Graph           `json:"graph"`
	StartupPlan *graph.StartupPlan     `json:"startup_plan"`
	Linter      *linter.Result         `json:"linter"`
	Mermaid     string                 `json:"mermaid"`
	DOT         string                 `json:"dot"`
}

// GenerateHTML produces a single self-contained HTML file for offline exploration.
func GenerateHTML(cfg *compose.ComposeConfig, g *graph.Graph, plan *graph.StartupPlan, linterRes *linter.Result) (string, error) {
	mermaidStr := ""
	dotStr := ""
	if g != nil {
		mermaidStr = g.ToMermaid()
		dotStr = g.ToDOT()
	}

	data := ReportData{
		Compose:     cfg,
		Graph:       g,
		StartupPlan: plan,
		Linter:      linterRes,
		Mermaid:     mermaidStr,
		DOT:         dotStr,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to serialize report data: %w", err)
	}

	pageTitle := "StackLoom Topology and Linter Report"
	if cfg != nil && cfg.Name != "" {
		pageTitle = fmt.Sprintf("StackLoom - %s", cfg.Name)
	}

	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>` + html.EscapeString(pageTitle) + `</title>
  <style>
    :root {
      --bg: #090d16;
      --card-bg: #111827;
      --card-border: #1f2937;
      --text: #f3f4f6;
      --muted: #9ca3af;
      --primary: #3b82f6;
      --critical: #ef4444;
      --warning: #f59e0b;
      --security: #8b5cf6;
      --healthy: #10b981;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background-color: var(--bg);
      color: var(--text);
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      line-height: 1.5;
      display: flex;
      flex-direction: column;
      height: 100vh;
      overflow: hidden;
    }
    header {
      background: var(--card-bg);
      border-bottom: 1px solid var(--card-border);
      padding: 12px 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 10px;
      font-weight: 700;
      font-size: 1.15rem;
      letter-spacing: -0.025em;
    }
    .brand-badge {
      font-size: 0.7rem;
      background: #1e3a8a;
      color: #93c5fd;
      padding: 2px 8px;
      border-radius: 9999px;
      text-transform: uppercase;
      font-weight: 600;
    }
    nav {
      display: flex;
      gap: 8px;
    }
    .nav-btn {
      background: transparent;
      border: 1px solid var(--card-border);
      color: var(--muted);
      padding: 6px 14px;
      border-radius: 6px;
      cursor: pointer;
      font-size: 0.875rem;
      transition: all 0.15s ease;
    }
    .nav-btn:hover, .nav-btn.active {
      background: #1f2937;
      color: #fff;
      border-color: #374151;
    }
    .nav-btn .pill {
      font-size: 0.75rem;
      padding: 2px 6px;
      border-radius: 9999px;
      margin-left: 6px;
      background: #374151;
    }
    .pill.crit { background: rgba(239, 68, 68, 0.2); color: #f87171; }
    .pill.warn { background: rgba(245, 158, 11, 0.2); color: #fbbf24; }
    main {
      flex: 1;
      display: flex;
      overflow: hidden;
      position: relative;
    }
    .view-container {
      display: none;
      flex: 1;
      overflow: auto;
      padding: 24px;
    }
    .view-container.active {
      display: block;
    }
    #view-graph {
      display: none;
      flex: 1;
      position: relative;
      overflow: hidden;
      background: radial-gradient(circle at 1px 1px, #1e293b 1px, transparent 0);
      background-size: 24px 24px;
    }
    #view-graph.active {
      display: flex;
    }
    .graph-canvas {
      flex: 1;
      overflow: auto;
      padding: 40px;
      display: flex;
      gap: 48px;
      align-items: flex-start;
      justify-content: center;
    }
    .stage-column {
      display: flex;
      flex-direction: column;
      gap: 20px;
      min-width: 240px;
    }
    .stage-header {
      font-size: 0.75rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--muted);
      border-bottom: 1px solid var(--card-border);
      padding-bottom: 6px;
      margin-bottom: 4px;
    }
    .node-card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 8px;
      padding: 14px;
      cursor: pointer;
      transition: all 0.2s ease;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.2);
    }
    .node-card:hover {
      border-color: var(--primary);
      transform: translateY(-2px);
    }
    .node-card.selected {
      border-color: var(--primary);
      box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.3);
    }
    .node-title {
      font-weight: 600;
      font-size: 0.95rem;
      margin-bottom: 4px;
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .node-image {
      font-size: 0.75rem;
      color: var(--muted);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      margin-bottom: 10px;
    }
    .node-tags {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
    }
    .tag {
      font-size: 0.7rem;
      padding: 2px 6px;
      border-radius: 4px;
      background: #1f2937;
      color: #94a3b8;
    }
    .tag.port {
      background: #064e3b;
      color: #6ee7b7;
    }
    .inspector-panel {
      width: 380px;
      background: var(--card-bg);
      border-left: 1px solid var(--card-border);
      overflow-y: auto;
      padding: 24px;
      display: none;
    }
    .inspector-panel.open {
      display: block;
    }
    .panel-section {
      margin-bottom: 20px;
    }
    .panel-section h3 {
      font-size: 0.8rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--muted);
      margin-bottom: 8px;
    }
    .finding-card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 8px;
      padding: 16px;
      margin-bottom: 16px;
    }
    .finding-header {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 8px;
    }
    .severity-pill {
      font-size: 0.7rem;
      font-weight: 700;
      padding: 2px 8px;
      border-radius: 4px;
      text-transform: uppercase;
    }
    .severity-pill.CRITICAL { background: #7f1d1d; color: #fca5a5; }
    .severity-pill.WARNING { background: #78350f; color: #fcd34d; }
    .severity-pill.SECURITY { background: #581c87; color: #d8b4fe; }
    pre {
      background: #030712;
      border: 1px solid var(--card-border);
      padding: 12px;
      border-radius: 6px;
      overflow-x: auto;
      font-size: 0.8rem;
      font-family: monospace;
      color: #cbd5e1;
      margin-top: 8px;
    }
  </style>
</head>
<body>
  <header>
    <div class="brand">
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#3b82f6" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path>
        <polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline>
        <line x1="12" y1="22.08" x2="12" y2="12"></line>
      </svg>
      <span>StackLoom</span>
      <span class="brand-badge">Topology Studio</span>
    </div>
    <nav>
      <button class="nav-btn active" onclick="switchTab('graph')">Topology DAG</button>
      <button class="nav-btn" onclick="switchTab('linter')">
        Pre-Flight Linter
        <span id="linter-pill" class="pill">0</span>
      </button>
      <button class="nav-btn" onclick="switchTab('startup')">Startup Waves</button>
      <button class="nav-btn" onclick="switchTab('diagrams')">Diagram Code</button>
    </nav>
  </header>

  <main>
    <div id="view-graph" class="active">
      <div id="graph-canvas" class="graph-canvas"></div>
      <aside id="inspector" class="inspector-panel">
        <h2 id="inspect-title" style="margin-bottom: 4px;">Service Details</h2>
        <div id="inspect-content"></div>
      </aside>
    </div>

    <div id="view-linter" class="view-container">
      <div style="max-width: 900px; margin: 0 auto;">
        <h2 style="margin-bottom: 16px;">Pre-Flight Static Linter Diagnostics</h2>
        <div id="linter-list"></div>
      </div>
    </div>

    <div id="view-startup" class="view-container">
      <div style="max-width: 800px; margin: 0 auto;">
        <h2 style="margin-bottom: 16px;">Topological Startup Plan (Kahn's Algorithm)</h2>
        <div id="startup-waves"></div>
      </div>
    </div>

    <div id="view-diagrams" class="view-container">
      <div style="max-width: 900px; margin: 0 auto;">
        <h2 style="margin-bottom: 12px;">Mermaid Flowchart Syntax</h2>
        <pre><code id="mermaid-code"></code></pre>
        <h2 style="margin-top: 24px; margin-bottom: 12px;">Graphviz DOT Syntax</h2>
        <pre><code id="dot-code"></code></pre>
      </div>
    </div>
  </main>

  <script id="stackloom-data" type="application/json">
` + string(dataBytes) + `
  </script>

  <script>
    const data = JSON.parse(document.getElementById('stackloom-data').textContent);
    let selectedNode = null;

    function init() {
      // Initialize linter pill
      const pill = document.getElementById('linter-pill');
      const count = (data.linter?.findings || []).length;
      pill.textContent = count;
      if (data.linter?.critical_count > 0) {
        pill.className = 'pill crit';
      } else if (data.linter?.warning_count > 0) {
        pill.className = 'pill warn';
      }

      renderGraph();
      renderLinter();
      renderStartupWaves();
      renderDiagrams();
    }

    function switchTab(tab) {
      document.querySelectorAll('nav .nav-btn').forEach(btn => btn.classList.remove('active'));
      document.querySelectorAll('main > div').forEach(div => div.classList.remove('active'));

      const eventTarget = window.event?.currentTarget;
      if (eventTarget) eventTarget.classList.add('active');

      const targetView = document.getElementById('view-' + tab);
      if (targetView) targetView.classList.add('active');
    }

    function renderGraph() {
      const canvas = document.getElementById('graph-canvas');
      canvas.innerHTML = '';

      const stages = data.startup_plan?.stages || [Object.keys(data.graph?.nodes || {})];

      stages.forEach((stageServices, stageIdx) => {
        const col = document.createElement('div');
        col.className = 'stage-column';
        col.innerHTML = '<div class="stage-header">Wave ' + stageIdx + ' (' + stageServices.length + ' services)</div>';

        stageServices.forEach(sName => {
          const node = data.graph?.nodes[sName];
          if (!node) return;

          const card = document.createElement('div');
          card.className = 'node-card' + (selectedNode === sName ? ' selected' : '');
          card.onclick = () => selectService(sName);

          const ports = (node.ports || []).map(p => '<span class="tag port">' + (p.published ? p.published + ':' : '') + p.target + '</span>').join('');
          const networks = (node.networks || []).map(n => '<span class="tag">' + n + '</span>').join('');

          card.innerHTML = 
            '<div class="node-title">' +
              '<span>' + node.name + '</span>' +
              (node.has_healthcheck ? '<span title="Healthcheck active" style="color: var(--healthy); font-size: 0.75rem;">✓ health</span>' : '') +
            '</div>' +
            '<div class="node-image" title="' + (node.image || '') + '">' + (node.image || 'build') + '</div>' +
            '<div class="node-tags">' + ports + networks + '</div>';

          col.appendChild(card);
        });

        canvas.appendChild(col);
      });
    }

    function selectService(name) {
      selectedNode = name;
      renderGraph();

      const inspector = document.getElementById('inspector');
      const title = document.getElementById('inspect-title');
      const content = document.getElementById('inspect-content');

      const node = data.graph?.nodes[name];
      if (!node) {
        inspector.classList.remove('open');
        return;
      }

      inspector.classList.add('open');
      title.textContent = node.name;

      const upstream = (data.graph?.adj && data.graph.adj[name]) || [];
      const downstream = (data.graph?.rev_adj && data.graph.rev_adj[name]) || [];

      let html = 
        '<div class="panel-section">' +
          '<h3>Image</h3><p style="font-family: monospace; font-size: 0.85rem;">' + (node.image || 'Local Dockerfile Build') + '</p>' +
        '</div>' +
        '<div class="panel-section">' +
          '<h3>Published Ports</h3><p>' + ((node.ports || []).map(p => p.raw).join(', ') || 'None') + '</p>' +
        '</div>' +
        '<div class="panel-section">' +
          '<h3>Direct Dependencies (Awaited)</h3>' +
          (upstream.length ? upstream.map(u => '<span class="tag" style="margin-right: 4px;">' + u + '</span>').join('') : '<p style="color: var(--muted); font-size: 0.85rem;">Independent service</p>') +
        '</div>' +
        '<div class="panel-section">' +
          '<h3>Dependents (Waiting for this)</h3>' +
          (downstream.length ? downstream.map(d => '<span class="tag" style="margin-right: 4px;">' + d + '</span>').join('') : '<p style="color: var(--muted); font-size: 0.85rem;">None</p>') +
        '</div>';

      content.innerHTML = html;
    }

    function renderLinter() {
      const list = document.getElementById('linter-list');
      const findings = data.linter?.findings || [];
      if (findings.length === 0) {
        list.innerHTML = '<div class="finding-card" style="border-color: var(--healthy); color: var(--healthy);">✓ All pre-flight checks passed! Zero port conflicts or dependency errors found.</div>';
        return;
      }

      list.innerHTML = findings.map(f => 
        '<div class="finding-card">' +
          '<div class="finding-header">' +
            '<span class="severity-pill ' + f.severity + '">' + f.severity + '</span>' +
            '<strong>' + f.rule_id + '</strong>' +
            (f.service ? '<span class="tag">' + f.service + '</span>' : '') +
          '</div>' +
          '<div style="font-size: 0.95rem; margin-bottom: 6px;">' + f.message + '</div>' +
          (f.detail ? '<div style="font-size: 0.8rem; color: var(--muted); margin-bottom: 8px;">' + f.detail + '</div>' : '') +
          '<div style="font-size: 0.85rem; color: #93c5fd; margin-bottom: 4px;"><strong>Recommendation:</strong> ' + f.recommendation + '</div>' +
          (f.suggested_override ? '<pre><code>' + f.suggested_override + '</code></pre>' : '') +
        '</div>'
      ).join('');
    }

    function renderStartupWaves() {
      const container = document.getElementById('startup-waves');
      const stages = data.startup_plan?.stages || [];

      if (stages.length === 0) {
        container.innerHTML = '<p>No startup order available.</p>';
        return;
      }

      container.innerHTML = stages.map((stage, idx) => 
        '<div class="finding-card">' +
          '<div style="font-weight: 600; margin-bottom: 8px;">Wave ' + idx + ' (Parallel Startup)</div>' +
          '<div style="display: flex; gap: 8px; flex-wrap: wrap;">' +
            stage.map(s => '<span class="tag" style="font-size: 0.85rem; padding: 4px 8px;">' + s + '</span>').join('') +
          '</div>' +
        '</div>'
      ).join('');
    }

    function renderDiagrams() {
      document.getElementById('mermaid-code').textContent = data.mermaid || '';
      document.getElementById('dot-code').textContent = data.dot || '';
    }

    window.onload = init;
  </script>
</body>
</html>`

	return tmpl, nil
}
