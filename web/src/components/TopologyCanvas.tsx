import React, { useMemo } from 'react';
import {
  ReactFlow,
  Controls,
  Background,
  MiniMap,
  Node,
  Edge,
  MarkerType,
  Position,
  ConnectionLineType,
  BackgroundVariant,
} from '@xyflow/react';
import dagre from '@dagrejs/dagre';
import { ServiceNode } from './ServiceNode';
import { GraphData, StartupPlan, BlastRadiusReport, EndpointProbe } from '../types';

interface TopologyCanvasProps {
  graph: GraphData | null;
  startupPlan: StartupPlan | null;
  metricsMap: Record<string, any>;
  collisionPorts: Set<number>;
  blastRadius: BlastRadiusReport | null;
  activeNetwork: string | null;
  probesMap: Record<string, EndpointProbe>;
  direction?: 'LR' | 'TB';
  selectedService: string | null;
  onSelectService: (service: string | null) => void;
  onOpenLogs: (service: string) => void;
  onRestartService: (service: string) => void;
  onSimulateOutage: (service: string) => void;
}

const nodeWidth = 300;
const nodeHeight = 250;

export const TopologyCanvas: React.FC<TopologyCanvasProps> = ({
  graph,
  metricsMap,
  collisionPorts,
  blastRadius,
  activeNetwork,
  probesMap,
  direction = 'LR',
  selectedService,
  onSelectService,
  onOpenLogs,
  onRestartService,
  onSimulateOutage,
}) => {
  const nodeTypes = useMemo(() => ({ serviceNode: ServiceNode }), []);

  const { nodes, edges } = useMemo(() => {
    if (!graph || Object.keys(graph.nodes).length === 0) {
      return { nodes: [], edges: [] };
    }

    const dagreGraph = new dagre.graphlib.Graph();
    dagreGraph.setDefaultEdgeLabel(() => ({}));
    dagreGraph.setGraph({
      rankdir: direction,
      nodesep: 50,
      ranksep: 90,
      marginx: 40,
      marginy: 40,
    });

    const isHorizontal = direction === 'LR';

    // Add nodes to dagre
    Object.values(graph.nodes).forEach((n) => {
      dagreGraph.setNode(n.id, { width: nodeWidth, height: nodeHeight });
    });

    // Add edges to dagre (flow from dependency to dependent)
    (graph.edges || []).forEach((e) => {
      dagreGraph.setEdge(e.to, e.from);
    });

    dagre.layout(dagreGraph);

    // Map of blast radius impacts
    const impactMap = new Map<string, { depth: number; severity: 'FATAL' | 'DEGRADED' }>();
    if (blastRadius) {
      (blastRadius.impact_tree || []).forEach((it) => {
        impactMap.set(it.service, { depth: it.depth, severity: it.severity });
      });
    }

    // Build React Flow nodes
    const flowNodes: Node[] = Object.values(graph.nodes).map((n) => {
      const nodePos = dagreGraph.node(n.id);
      const metrics = metricsMap[n.name];
      const probe = probesMap[n.name];

      // Check collision ports
      let hasPortCollision = false;
      (n.ports || []).forEach((p) => {
        if (p.published && collisionPorts.has(p.published)) {
          hasPortCollision = true;
        }
      });

      // Blast radius state
      const isOutageTarget = blastRadius?.target_service === n.name;
      const impact = impactMap.get(n.name);
      const isOutageImpacted = Boolean(impact);

      // Network filter dimming
      let isDimmed = false;
      if (activeNetwork) {
        const nets = n.networks || ['default'];
        if (!nets.includes(activeNetwork)) {
          isDimmed = true;
        }
      }
      if (blastRadius && !isOutageTarget && !isOutageImpacted) {
        isDimmed = true;
      }

      return {
        id: n.id,
        type: 'serviceNode',
        position: {
          x: nodePos ? nodePos.x - nodeWidth / 2 : 0,
          y: nodePos ? nodePos.y - nodeHeight / 2 : 0,
        },
        data: {
          ...n,
          metrics,
          probe,
          hasPortCollision,
          isOutageTarget,
          isOutageImpacted,
          outageDepth: impact?.depth,
          outageSeverity: impact?.severity,
          isDimmed,
          onOpenLogs,
          onRestart: onRestartService,
          onSimulateOutage,
        },
        selected: selectedService === n.id,
        targetPosition: isHorizontal ? Position.Left : Position.Top,
        sourcePosition: isHorizontal ? Position.Right : Position.Bottom,
      };
    });

    // Build React Flow edges
    const flowEdges: Edge[] = (graph.edges || []).map((e, idx) => {
      const isHealthyCond = e.condition === 'service_healthy';
      const isImpactedEdge =
        blastRadius &&
        (e.to === blastRadius.target_service || impactMap.has(e.to)) &&
        impactMap.has(e.from);

      return {
        id: `e-${e.from}-${e.to}-${idx}`,
        source: e.to,
        target: e.from,
        animated: isHealthyCond || Boolean(isImpactedEdge),
        type: ConnectionLineType.SmoothStep,
        style: {
          stroke: isImpactedEdge ? '#ef4444' : isHealthyCond ? '#10b981' : '#3b82f6',
          strokeWidth: isImpactedEdge ? 3 : 2,
        },
        label: isImpactedEdge ? 'CRASH RISK' : isHealthyCond ? 'healthy' : undefined,
        labelStyle: { fill: isImpactedEdge ? '#fca5a5' : '#94a3b8', fontSize: 10, fontWeight: 600 },
        labelBgStyle: { fill: isImpactedEdge ? '#450a0a' : '#0f172a', rx: 4, ry: 4 },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: isImpactedEdge ? '#ef4444' : isHealthyCond ? '#10b981' : '#3b82f6',
          width: 14,
          height: 14,
        },
      };
    });

    return { nodes: flowNodes, edges: flowEdges };
  }, [
    graph,
    metricsMap,
    collisionPorts,
    blastRadius,
    activeNetwork,
    probesMap,
    direction,
    selectedService,
    onOpenLogs,
    onRestartService,
    onSimulateOutage,
  ]);

  return (
    <div className="w-full h-full relative bg-[#090d16]" id="topology-canvas-container">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        onNodeClick={(_, node) => onSelectService(node.id)}
        onPaneClick={() => onSelectService(null)}
        proOptions={{ hideAttribution: true }}
      >
        <Background variant={BackgroundVariant.Dots} gap={24} size={1.2} color="#1e293b" />
        <Controls className="!bg-[#111827] !border !border-slate-800 !rounded-lg !overflow-hidden [&>button]:!border-slate-800 [&>button]:!bg-[#111827] [&>button]:!text-slate-300" />
        <MiniMap
          nodeColor="#1e293b"
          maskColor="rgba(9, 13, 22, 0.7)"
          className="!bg-[#111827] !border !border-slate-800 !rounded-lg overflow-hidden"
        />
      </ReactFlow>
    </div>
  );
};
