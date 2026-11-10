import React from 'react';
import {
  Play,
  Square,
  Download,
  ShieldCheck,
  AlertCircle,
  ArrowRightLeft,
  Database,
  Network,
  Image,
  Flame,
  X,
} from 'lucide-react';
import { LinterResult, BlastRadiusReport } from '../types';

interface SubGraphControlsProps {
  allProfiles: string[];
  activeProfiles: string[];
  onToggleProfile: (profile: string) => void;
  networks: string[];
  activeNetwork: string | null;
  onSelectNetwork: (network: string | null) => void;
  selectedService: string | null;
  blastRadius: BlastRadiusReport | null;
  onClearOutage: () => void;
  onRunSubgraph: (services: string[], action: 'start' | 'stop') => void;
  onStartDatabasesOnly: () => void;
  onExportHTML: () => void;
  onExportPNG: () => void;
  onExportSVG: () => void;
  onOpenLinter: () => void;
  linterResult: LinterResult | null;
  direction: 'LR' | 'TB';
  onToggleDirection: () => void;
  searchQuery: string;
  onSearchChange: (q: string) => void;
}

export const SubGraphControls: React.FC<SubGraphControlsProps> = ({
  allProfiles,
  activeProfiles,
  onToggleProfile,
  networks,
  activeNetwork,
  onSelectNetwork,
  selectedService,
  blastRadius,
  onClearOutage,
  onRunSubgraph,
  onStartDatabasesOnly,
  onExportHTML,
  onExportPNG,
  onExportSVG,
  onOpenLinter,
  linterResult,
  direction,
  onToggleDirection,
  searchQuery,
  onSearchChange,
}) => {
  const critCount = linterResult?.critical_count ?? 0;
  const warnCount = linterResult?.warning_count ?? 0;

  return (
    <div className="h-12 bg-[#111827] border-b border-slate-800 px-4 flex items-center justify-between gap-3 text-xs">
      {/* Left: Quick Actions, Profiles & Active Outage Banner */}
      <div className="flex items-center gap-2">
        {blastRadius ? (
          <div className="flex items-center gap-2 bg-red-950/70 border border-red-700/80 px-2.5 py-1 rounded-md text-red-200">
            <Flame className="w-3.5 h-3.5 text-red-400" />
            <span>
              Outage on <strong>{blastRadius.target_service}</strong>: {blastRadius.affected_count}/{blastRadius.total_services} services impacted ({(blastRadius.blast_radius_ratio * 100).toFixed(0)}%)
            </span>
            <button
              onClick={onClearOutage}
              className="p-0.5 rounded hover:bg-red-900/80 text-red-300 hover:text-white transition-colors ml-1"
              title="Clear Outage Simulation"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        ) : (
          <>
            <button
              onClick={onStartDatabasesOnly}
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md bg-slate-800 hover:bg-slate-700 text-slate-200 transition-colors border border-slate-700"
              title="Start only database and cache services to save RAM"
            >
              <Database className="w-3.5 h-3.5 text-blue-400" />
              <span>DBs & Cache Only</span>
            </button>

            {selectedService && (
              <button
                onClick={() => onRunSubgraph([selectedService], 'start')}
                className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md bg-blue-600 hover:bg-blue-500 text-white transition-colors"
              >
                <Play className="w-3.5 h-3.5" />
                <span>Run {selectedService} + Deps</span>
              </button>
            )}

            <button
              onClick={() => onRunSubgraph([], 'stop')}
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors border border-slate-700"
              title="Teardown All Services"
            >
              <Square className="w-3.5 h-3.5 text-red-400" />
              <span>Stop All</span>
            </button>
          </>
        )}

        {/* Profile Chips */}
        {allProfiles.length > 0 && (
          <div className="flex items-center gap-1 ml-2 pl-2 border-l border-slate-800">
            <span className="text-slate-400 text-[11px]">Profiles:</span>
            {allProfiles.map((prof) => {
              const isActive = activeProfiles.includes(prof);
              return (
                <button
                  key={prof}
                  onClick={() => onToggleProfile(prof)}
                  className={`px-2 py-0.5 rounded text-[11px] font-mono transition-colors border ${
                    isActive
                      ? 'bg-amber-950/70 border-amber-700 text-amber-300'
                      : 'bg-slate-800/60 border-slate-700 text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {prof}
                </button>
              );
            })}
          </div>
        )}

        {/* Network Filter */}
        {networks.length > 0 && (
          <div className="flex items-center gap-1 ml-2 pl-2 border-l border-slate-800">
            <Network className="w-3.5 h-3.5 text-slate-400" />
            <select
              value={activeNetwork || ''}
              onChange={(e) => onSelectNetwork(e.target.value || null)}
              className="bg-slate-900 border border-slate-700 rounded px-2 py-0.5 text-slate-200 text-[11px] focus:outline-none"
            >
              <option value="">All Networks</option>
              {networks.map((net) => (
                <option key={net} value={net}>
                  Net: {net}
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      {/* Right: Layout, Search, Linter Pill, Export Menus */}
      <div className="flex items-center gap-2">
        {/* Search */}
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Filter nodes..."
          className="w-32 px-2.5 py-1 bg-slate-900 border border-slate-800 rounded-md text-slate-200 placeholder:text-slate-500 focus:outline-none focus:border-blue-500 text-xs"
        />

        {/* Layout direction */}
        <button
          onClick={onToggleDirection}
          className="p-1.5 rounded-md bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition-colors"
          title={`Switch layout to ${direction === 'LR' ? 'Top-to-Bottom' : 'Left-to-Right'}`}
        >
          <ArrowRightLeft className="w-3.5 h-3.5" />
        </button>

        {/* Linter Pill */}
        <button
          onClick={onOpenLinter}
          className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md border text-xs font-medium transition-colors ${
            critCount > 0
              ? 'bg-red-950/60 border-red-800/80 text-red-300'
              : warnCount > 0
              ? 'bg-amber-950/60 border-amber-800/80 text-amber-300'
              : 'bg-emerald-950/40 border-emerald-800/60 text-emerald-300'
          }`}
        >
          {critCount > 0 ? (
            <AlertCircle className="w-3.5 h-3.5 text-red-400" />
          ) : (
            <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
          )}
          <span>
            {critCount > 0
              ? `${critCount} Critical`
              : warnCount > 0
              ? `${warnCount} Warnings`
              : 'Pre-Flight OK'}
          </span>
        </button>

        {/* Image / HTML Export Dropdowns */}
        <div className="flex items-center rounded-md border border-slate-700 bg-slate-800 overflow-hidden">
          <button
            onClick={onExportPNG}
            className="px-2 py-1 text-slate-300 hover:text-white hover:bg-slate-700 transition-colors flex items-center gap-1"
            title="Export high-resolution PNG image"
          >
            <Image className="w-3.5 h-3.5 text-emerald-400" />
            <span>PNG</span>
          </button>
          <button
            onClick={onExportSVG}
            className="px-2 py-1 text-slate-300 hover:text-white hover:bg-slate-700 transition-colors border-l border-slate-700"
            title="Export vector SVG image"
          >
            <span>SVG</span>
          </button>
          <button
            onClick={onExportHTML}
            className="px-2 py-1 text-slate-300 hover:text-white hover:bg-slate-700 transition-colors border-l border-slate-700 flex items-center gap-1"
            title="Download standalone offline interactive HTML report"
          >
            <Download className="w-3.5 h-3.5 text-blue-400" />
            <span>HTML</span>
          </button>
        </div>
      </div>
    </div>
  );
};
