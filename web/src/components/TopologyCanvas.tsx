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
import { GraphData, StartupPlan } from '../types';

interface TopologyCanvasProps {
  graph: GraphData | null;
  startupPlan: StartupPlan | null;
  metricsMap: Record<string, any>;
  collisionPorts: Set<number>;
  direction?: 'LR' | 'TB';
  selectedService: string | null;
  onSelectService: (service: string | null) => void;
  onOpenLogs: (service: string) => void;
  onRestartService: (service: string) => void;
}

const nodeWidth = 300;
const nodeHeight = 220;

export const TopologyCanvas: React.FC<TopologyCanvasProps> = ({
  graph,
  metricsMap,
  collisionPorts,
  direction = 'LR',
  selectedService,
  onSelectService,
  onOpenLogs,
  onRestartService,
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

    // Add edges to dagre
    // Edge is from dependent (From) -> dependency (To)
    // In visual layout, dependency is upstream (to the left/top), dependent is downstream (to the right/bottom)
    (graph.edges || []).forEach((e) => {
      dagreGraph.setEdge(e.to, e.from);
    });

    dagre.layout(dagreGraph);

    // Build React Flow nodes
    const flowNodes: Node[] = Object.values(graph.nodes).map((n) => {
      const nodePos = dagreGraph.node(n.id);
      const metrics = metricsMap[n.name];

      // Check if service has any collision ports
      let hasPortCollision = false;
      (n.ports || []).forEach((p) => {
        if (p.published && collisionPorts.has(p.published)) {
          hasPortCollision = true;
        }
      });

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
          hasPortCollision,
          onOpenLogs,
          onRestart: onRestartService,
        },
        selected: selectedService === n.id,
        targetPosition: isHorizontal ? Position.Left : Position.Top,
        sourcePosition: isHorizontal ? Position.Right : Position.Bottom,
      };
    });

    // Build React Flow edges
    const flowEdges: Edge[] = (graph.edges || []).map((e, idx) => {
      const isHealthyCond = e.condition === 'service_healthy';
      return {
        id: `e-${e.from}-${e.to}-${idx}`,
        source: e.to, // Flow from dependency -> dependent
        target: e.from,
        animated: isHealthyCond,
        type: ConnectionLineType.SmoothStep,
        style: {
          stroke: isHealthyCond ? '#10b981' : '#3b82f6',
          strokeWidth: 2,
        },
        label: isHealthyCond ? 'healthy' : undefined,
        labelStyle: { fill: '#94a3b8', fontSize: 10, fontWeight: 500 },
        labelBgStyle: { fill: '#0f172a', rx: 4, ry: 4 },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: isHealthyCond ? '#10b981' : '#3b82f6',
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
    direction,
    selectedService,
    onOpenLogs,
    onRestartService,
  ]);

  return (
    <div className="w-full h-full relative bg-[#090d16]">
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
