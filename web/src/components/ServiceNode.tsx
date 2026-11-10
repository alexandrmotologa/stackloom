import React from 'react';
import { Handle, Position, NodeProps, Node } from '@xyflow/react';
import { ServiceNodeData } from '../types';
import { Activity, AlertTriangle, CheckCircle2, RotateCw, Terminal, ShieldAlert } from 'lucide-react';

export type ServiceNodeType = Node<
  ServiceNodeData & {
    onOpenLogs?: (service: string) => void;
    onRestart?: (service: string) => void;
  },
  'serviceNode'
>;

export const ServiceNode: React.FC<NodeProps<ServiceNodeType>> = ({ data, selected }) => {
  const isRunning = data.metrics?.state === 'running';
  const isHealthy = data.metrics?.health === 'healthy';
  const isUnhealthy = data.metrics?.health === 'unhealthy';
  const cpuPercent = data.metrics?.cpu_percentage ?? 0;
  const memUsageMB = data.metrics?.memory_usage_mb ?? 0;
  const memLimitMB = data.metrics?.memory_limit_mb ?? 0;
  const memPercent = data.metrics?.memory_percent ?? 0;

  return (
    <div
      className={`w-72 bg-[#111827] border rounded-xl p-3.5 shadow-xl transition-all duration-200 select-none ${
        selected
          ? 'border-blue-500 ring-2 ring-blue-500/20 shadow-blue-500/10'
          : data.hasPortCollision
          ? 'border-red-500/60 bg-red-950/10'
          : 'border-slate-800 hover:border-slate-700'
      }`}
    >
      {/* Dependency Handles */}
      <Handle
        type="target"
        position={Position.Left}
        className="!bg-blue-500 !border-2 !border-[#090d16]"
      />

      {/* Header */}
      <div className="flex items-center justify-between gap-2 mb-2">
        <div className="flex items-center gap-2 min-w-0">
          <div
            className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${
              isRunning
                ? isHealthy
                  ? 'bg-emerald-400 animate-pulse'
                  : isUnhealthy
                  ? 'bg-red-400'
                  : 'bg-emerald-400'
                : 'bg-slate-500'
            }`}
          />
          <span className="font-semibold text-sm text-slate-100 truncate tracking-tight">
            {data.name}
          </span>
        </div>

        <div className="flex items-center gap-1">
          {data.hasPortCollision && (
            <span title="Port Collision Detected" className="text-red-400">
              <AlertTriangle className="w-3.5 h-3.5" />
            </span>
          )}
          {data.privileged && (
            <span title="Privileged Mode Enabled" className="text-purple-400">
              <ShieldAlert className="w-3.5 h-3.5" />
            </span>
          )}
          {data.has_healthcheck && (
            <span title="Healthcheck Configured" className="text-emerald-400">
              <CheckCircle2 className="w-3.5 h-3.5" />
            </span>
          )}
        </div>
      </div>

      {/* Image / Build context */}
      <div className="text-[11px] text-slate-400 truncate mb-3 font-mono bg-slate-900/60 px-2 py-1 rounded border border-slate-800/80">
        {data.image || 'Local Dockerfile'}
      </div>

      {/* Real-time Metrics (if running) */}
      {isRunning ? (
        <div className="space-y-1.5 mb-3 bg-slate-900/40 p-2 rounded-lg border border-slate-800/50">
          <div className="flex justify-between text-[10px] text-slate-400">
            <span className="flex items-center gap-1">
              <Activity className="w-2.5 h-2.5 text-blue-400" /> CPU
            </span>
            <span className="font-mono text-slate-200">{cpuPercent.toFixed(1)}%</span>
          </div>
          <div className="w-full bg-slate-800 h-1 rounded-full overflow-hidden">
            <div
              className={`h-full transition-all duration-500 ${
                cpuPercent > 80 ? 'bg-red-500' : cpuPercent > 50 ? 'bg-amber-400' : 'bg-blue-500'
              }`}
              style={{ width: `${Math.min(100, Math.max(2, cpuPercent))}%` }}
            />
          </div>

          <div className="flex justify-between text-[10px] text-slate-400 pt-0.5">
            <span>RAM</span>
            <span className="font-mono text-slate-200">
              {memUsageMB.toFixed(0)}MB {memLimitMB > 0 ? `/ ${(memLimitMB / 1024).toFixed(1)}GB` : ''}
            </span>
          </div>
          <div className="w-full bg-slate-800 h-1 rounded-full overflow-hidden">
            <div
              className="h-full bg-emerald-500 transition-all duration-500"
              style={{ width: `${Math.min(100, Math.max(2, memPercent))}%` }}
            />
          </div>
        </div>
      ) : (
        <div className="text-[10px] text-slate-500 italic mb-2.5">Container stopped / offline</div>
      )}

      {/* Ports and Networks */}
      <div className="flex flex-wrap gap-1.5 mb-3">
        {(data.ports || []).map((p, idx) => (
          <span
            key={idx}
            className={`text-[10px] font-mono px-1.5 py-0.5 rounded border ${
              data.hasPortCollision
                ? 'bg-red-950/40 border-red-800/80 text-red-300'
                : 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300'
            }`}
          >
            {p.published ? `${p.published}:` : ''}
            {p.target}
          </span>
        ))}
        {(data.networks || []).map((net, idx) => (
          <span
            key={idx}
            className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800/60 border border-slate-700/60 text-slate-300"
          >
            {net}
          </span>
        ))}
        {(data.profiles || []).map((prof, idx) => (
          <span
            key={idx}
            className="text-[10px] px-1.5 py-0.5 rounded bg-amber-950/40 border border-amber-800/60 text-amber-300"
          >
            prof:{prof}
          </span>
        ))}
      </div>

      {/* Actions Toolbar */}
      <div className="flex items-center justify-end gap-1.5 pt-2 border-t border-slate-800/80">
        <button
          onClick={(e) => {
            e.stopPropagation();
            data.onOpenLogs?.(data.name);
          }}
          className="flex items-center gap-1 text-[11px] px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition-colors"
          title="Live Container Logs"
        >
          <Terminal className="w-3 h-3" /> Logs
        </button>
        <button
          onClick={(e) => {
            e.stopPropagation();
            data.onRestart?.(data.name);
          }}
          className="flex items-center gap-1 text-[11px] px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition-colors"
          title="Restart Container"
        >
          <RotateCw className="w-3 h-3" /> Restart
        </button>
      </div>

      {/* Outbound Handle */}
      <Handle
        type="source"
        position={Position.Right}
        className="!bg-blue-500 !border-2 !border-[#090d16]"
      />
    </div>
  );
};
