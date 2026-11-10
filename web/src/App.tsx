import { useState, useEffect, useMemo, useCallback } from 'react';
import { TopologyCanvas } from './components/TopologyCanvas';
import { LinterDrawer } from './components/LinterDrawer';
import { LogViewerModal } from './components/LogViewerModal';
import { SubGraphControls } from './components/SubGraphControls';
import { SystemStatus, GraphData, StartupPlan, LinterResult } from './types';
import { Box, Layers, Cpu, Radio } from 'lucide-react';

export function App() {
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [graph, setGraph] = useState<GraphData | null>(null);
  const [startupPlan, setStartupPlan] = useState<StartupPlan | null>(null);
  const [linterResult, setLinterResult] = useState<LinterResult | null>(null);
  const [metricsMap, setMetricsMap] = useState<Record<string, any>>({});
  const [selectedService, setSelectedService] = useState<string | null>(null);
  const [activeServiceForLogs, setActiveServiceForLogs] = useState<string | null>(null);
  const [isLinterOpen, setIsLinterOpen] = useState(false);
  const [direction, setDirection] = useState<'LR' | 'TB'>('LR');
  const [searchQuery, setSearchQuery] = useState('');

  const fetchTopology = useCallback(async () => {
    try {
      const [statusRes, topoRes, lintRes] = await Promise.all([
        fetch('/api/status').then((r) => r.json()),
        fetch('/api/topology').then((r) => r.json()),
        fetch('/api/lint').then((r) => r.json()),
      ]);
      setStatus(statusRes);
      setGraph(topoRes.graph);
      setStartupPlan(topoRes.startup_plan);
      setLinterResult(lintRes);
    } catch (e) {
      console.error('Failed to load topology data', e);
    }
  }, []);

  useEffect(() => {
    fetchTopology();
  }, [fetchTopology]);

  // Connect to live stats WebSocket
  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/stats`;
    let ws: WebSocket | null = null;
    let reconnectTimeout: any = null;

    const connect = () => {
      ws = new WebSocket(wsUrl);
      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.services) {
            setMetricsMap(data.services);
          }
        } catch (e) {
          console.error('Error parsing stats payload', e);
        }
      };
      ws.onclose = () => {
        reconnectTimeout = setTimeout(connect, 3000);
      };
    };

    connect();

    return () => {
      if (ws) ws.close();
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
    };
  }, []);

  // Compute set of published ports flagged in linter as collisions
  const collisionPorts = useMemo(() => {
    const set = new Set<number>();
    (linterResult?.findings || []).forEach((f) => {
      if (f.rule_id === 'ERR_PORT_COLLISION') {
        const matches = f.message.match(/port (\d+)/);
        if (matches && matches[1]) {
          set.add(parseInt(matches[1], 10));
        }
      }
    });
    return set;
  }, [linterResult]);

  // Filter graph based on search query
  const filteredGraph = useMemo(() => {
    if (!graph || !searchQuery.trim()) return graph;
    const q = searchQuery.toLowerCase();
    const matchingNodes: Record<string, any> = {};

    Object.entries(graph.nodes).forEach(([k, v]) => {
      if (
        k.toLowerCase().includes(q) ||
        (v.image && v.image.toLowerCase().includes(q)) ||
        (v.ports && v.ports.some((p) => p.raw.includes(q)))
      ) {
        matchingNodes[k] = v;
      }
    });

    const matchingKeys = new Set(Object.keys(matchingNodes));
    const matchingEdges = (graph.edges || []).filter(
      (e) => matchingKeys.has(e.from) && matchingKeys.has(e.to)
    );

    return {
      nodes: matchingNodes,
      edges: matchingEdges,
    };
  }, [graph, searchQuery]);

  const handleToggleProfile = async (profile: string) => {
    if (!status) return;
    const current = new Set(status.active_profiles || []);
    if (current.has(profile)) {
      current.delete(profile);
    } else {
      current.add(profile);
    }

    try {
      await fetch('/api/profiles/active', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ profiles: Array.from(current) }),
      });
      fetchTopology();
    } catch (e) {
      console.error('Failed to toggle profile', e);
    }
  };

  const handleRestartService = async (service: string) => {
    try {
      await fetch('/api/actions/service', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ service, action: 'restart' }),
      });
    } catch (e) {
      console.error('Failed to restart service', e);
    }
  };

  const handleRunSubgraph = async (services: string[], action: 'start' | 'stop') => {
    try {
      await fetch('/api/actions/subgraph', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ services, action, upstream: true }),
      });
    } catch (e) {
      console.error('Failed to trigger sub-graph action', e);
    }
  };

  const handleStartDatabasesOnly = () => {
    if (!graph) return;
    const dbKeywords = ['db', 'postgres', 'mysql', 'mariadb', 'redis', 'mongo', 'memcached'];
    const dbServices = Object.keys(graph.nodes).filter((name) =>
      dbKeywords.some((k) => name.toLowerCase().includes(k))
    );
    if (dbServices.length > 0) {
      handleRunSubgraph(dbServices, 'start');
    }
  };

  const handleExportHTML = async () => {
    try {
      const html = await fetch('/api/export').then((r) => r.text());
      const blob = new Blob([html], { type: 'text/html' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `stackloom-topology-${status?.project_name || 'export'}.html`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (e) {
      console.error('Failed to export HTML', e);
    }
  };

  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden bg-[#090d16] text-slate-100">
      {/* Top Header */}
      <header className="h-14 bg-[#111827] border-b border-slate-800 px-5 flex items-center justify-between z-10 flex-shrink-0">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-blue-600/20 border border-blue-500/40 flex items-center justify-center text-blue-400">
            <Box className="w-4 h-4" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="font-bold text-sm tracking-tight text-slate-100">StackLoom</h1>
              <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-blue-950/80 text-blue-400 border border-blue-800/60 font-semibold">
                Studio
              </span>
            </div>
            <div className="text-[11px] text-slate-400 flex items-center gap-2">
              <span>{status?.project_name ? `Project: ${status.project_name}` : 'Compose Topology'}</span>
              <span>•</span>
              <span>{status?.service_count ?? 0} Services</span>
            </div>
          </div>
        </div>

        {/* Center: Startup Waves summary */}
        {startupPlan && startupPlan.stages.length > 0 && (
          <div className="hidden md:flex items-center gap-2 text-xs">
            <span className="text-slate-400 flex items-center gap-1">
              <Layers className="w-3.5 h-3.5 text-blue-400" />
              Startup Waves:
            </span>
            <div className="flex items-center gap-1.5">
              {startupPlan.stages.map((stage, idx) => (
                <span
                  key={idx}
                  className="px-2 py-0.5 rounded bg-slate-800/80 border border-slate-700 text-slate-300 font-mono text-[11px]"
                  title={`Stage ${idx}: ${stage.join(', ')}`}
                >
                  W{idx} ({stage.length})
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Right: Docker Connection Status */}
        <div className="flex items-center gap-3">
          <div
            className={`flex items-center gap-2 px-3 py-1 rounded-full border text-xs ${
              status?.docker_connected
                ? 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300'
                : 'bg-slate-800/60 border-slate-700 text-slate-400'
            }`}
          >
            <Radio
              className={`w-3.5 h-3.5 ${
                status?.docker_connected ? 'text-emerald-400 animate-pulse' : 'text-slate-500'
              }`}
            />
            <span>
              {status?.docker_connected
                ? 'Docker Socket Live'
                : 'Offline Static Studio'}
            </span>
          </div>
        </div>
      </header>

      {/* Sub-header Controls */}
      <SubGraphControls
        allProfiles={status?.all_profiles || []}
        activeProfiles={status?.active_profiles || []}
        onToggleProfile={handleToggleProfile}
        selectedService={selectedService}
        onRunSubgraph={handleRunSubgraph}
        onStartDatabasesOnly={handleStartDatabasesOnly}
        onExportHTML={handleExportHTML}
        onOpenLinter={() => setIsLinterOpen(true)}
        linterResult={linterResult}
        direction={direction}
        onToggleDirection={() => setDirection(direction === 'LR' ? 'TB' : 'LR')}
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
      />

      {/* Main Canvas Area */}
      <main className="flex-1 relative overflow-hidden">
        <TopologyCanvas
          graph={filteredGraph}
          startupPlan={startupPlan}
          metricsMap={metricsMap}
          collisionPorts={collisionPorts}
          direction={direction}
          selectedService={selectedService}
          onSelectService={setSelectedService}
          onOpenLogs={(s) => setActiveServiceForLogs(s)}
          onRestartService={handleRestartService}
        />

        {/* Linter Slide-over Drawer */}
        <LinterDrawer
          isOpen={isLinterOpen}
          onClose={() => setIsLinterOpen(false)}
          result={linterResult}
          onSelectService={(s) => {
            setSelectedService(s);
            setIsLinterOpen(false);
          }}
        />

        {/* Live Container Log Modal */}
        <LogViewerModal
          service={activeServiceForLogs}
          onClose={() => setActiveServiceForLogs(null)}
        />
      </main>

      {/* Bottom Status Bar */}
      <footer className="h-6 bg-[#0c1017] border-t border-slate-800/80 px-4 flex items-center justify-between text-[11px] text-slate-500 font-mono select-none">
        <div className="flex items-center gap-4">
          <span>Files: {(status?.compose_files || []).join(', ')}</span>
          {selectedService && (
            <span className="text-blue-400">Selected: {selectedService}</span>
          )}
        </div>
        <div className="flex items-center gap-3">
          <span className="flex items-center gap-1">
            <Cpu className="w-3 h-3 text-slate-400" /> &lt; 30MB RAM
          </span>
          <span>StackLoom v1.0.0</span>
        </div>
      </footer>
    </div>
  );
}
