import React, { useState } from 'react';
import { LinterResult, LinterFinding } from '../types';
import { X, AlertCircle, AlertTriangle, ShieldAlert, CheckCircle, Copy, Check, Wrench } from 'lucide-react';

interface LinterDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  result: LinterResult | null;
  onSelectService: (service: string) => void;
  onApplyFix?: (suggestedOverride: string) => Promise<boolean>;
}

export const LinterDrawer: React.FC<LinterDrawerProps> = ({
  isOpen,
  onClose,
  result,
  onSelectService,
  onApplyFix,
}) => {
  const [filter, setFilter] = useState<'ALL' | 'CRITICAL' | 'WARNING' | 'SECURITY'>('ALL');
  const [copiedIdx, setCopiedIdx] = useState<number | null>(null);
  const [applyingIdx, setApplyingIdx] = useState<number | null>(null);
  const [appliedIdx, setAppliedIdx] = useState<number | null>(null);

  if (!isOpen) return null;

  const findings = result?.findings || [];
  const filtered = findings.filter((f) => {
    if (filter === 'ALL') return true;
    return f.severity === filter;
  });

  const handleCopyOverride = (code: string, idx: number) => {
    navigator.clipboard.writeText(code);
    setCopiedIdx(idx);
    setTimeout(() => setCopiedIdx(null), 2000);
  };

  const handleApply = async (snippet: string, idx: number) => {
    if (!onApplyFix) return;
    setApplyingIdx(idx);
    const ok = await onApplyFix(snippet);
    setApplyingIdx(null);
    if (ok) {
      setAppliedIdx(idx);
      setTimeout(() => setAppliedIdx(null), 3000);
    }
  };

  return (
    <aside className="fixed inset-y-0 right-0 w-[480px] bg-[#111827] border-l border-slate-800 shadow-2xl z-50 flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-slate-800 flex items-center justify-between">
        <div>
          <h2 className="text-base font-semibold text-slate-100 flex items-center gap-2">
            <span>Pre-Flight Linter</span>
            <span
              className={`text-xs px-2 py-0.5 rounded-full font-bold ${
                result?.critical_count && result.critical_count > 0
                  ? 'bg-red-500/20 text-red-400'
                  : result?.warning_count && result.warning_count > 0
                  ? 'bg-amber-500/20 text-amber-400'
                  : 'bg-emerald-500/20 text-emerald-400'
              }`}
            >
              {findings.length} findings
            </span>
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            Static validation & 1-click auto-fixer
          </p>
        </div>

        <button
          onClick={onClose}
          className="p-1 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-white transition-colors"
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      {/* Filter Tabs */}
      <div className="flex gap-1.5 p-3 border-b border-slate-800/80 bg-slate-900/30 text-xs">
        <button
          onClick={() => setFilter('ALL')}
          className={`px-3 py-1.5 rounded-md font-medium transition-colors ${
            filter === 'ALL'
              ? 'bg-slate-800 text-white'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          All ({findings.length})
        </button>
        <button
          onClick={() => setFilter('CRITICAL')}
          className={`px-3 py-1.5 rounded-md font-medium flex items-center gap-1 transition-colors ${
            filter === 'CRITICAL'
              ? 'bg-red-950 text-red-200 border border-red-800/60'
              : 'text-red-400 hover:text-red-300'
          }`}
        >
          <AlertCircle className="w-3.5 h-3.5" />
          Critical ({result?.critical_count ?? 0})
        </button>
        <button
          onClick={() => setFilter('WARNING')}
          className={`px-3 py-1.5 rounded-md font-medium flex items-center gap-1 transition-colors ${
            filter === 'WARNING'
              ? 'bg-amber-950 text-amber-200 border border-amber-800/60'
              : 'text-amber-400 hover:text-amber-300'
          }`}
        >
          <AlertTriangle className="w-3.5 h-3.5" />
          Warning ({result?.warning_count ?? 0})
        </button>
        <button
          onClick={() => setFilter('SECURITY')}
          className={`px-3 py-1.5 rounded-md font-medium flex items-center gap-1 transition-colors ${
            filter === 'SECURITY'
              ? 'bg-purple-950 text-purple-200 border border-purple-800/60'
              : 'text-purple-400 hover:text-purple-300'
          }`}
        >
          <ShieldAlert className="w-3.5 h-3.5" />
          Security ({result?.security_count ?? 0})
        </button>
      </div>

      {/* Findings List */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3.5">
        {findings.length === 0 ? (
          <div className="p-6 text-center bg-emerald-950/20 border border-emerald-800/50 rounded-xl">
            <CheckCircle className="w-10 h-10 text-emerald-400 mx-auto mb-2" />
            <h3 className="font-semibold text-emerald-300 text-sm">All checks passed</h3>
            <p className="text-xs text-emerald-400/80 mt-1">
              Zero port collisions, dependency deadlocks, or uninitialized environment variables detected.
            </p>
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-8 text-xs text-slate-500">
            No findings for selected filter category.
          </div>
        ) : (
          filtered.map((f: LinterFinding, idx: number) => {
            const isCrit = f.severity === 'CRITICAL';
            const isWarn = f.severity === 'WARNING';
            const isSec = f.severity === 'SECURITY';

            return (
              <div
                key={idx}
                className={`p-3.5 rounded-xl border text-xs space-y-2.5 transition-all ${
                  isCrit
                    ? 'bg-red-950/20 border-red-900/60'
                    : isWarn
                    ? 'bg-amber-950/20 border-amber-900/60'
                    : 'bg-purple-950/20 border-purple-900/60'
                }`}
              >
                {/* Header */}
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5">
                    {isCrit && <AlertCircle className="w-4 h-4 text-red-400" />}
                    {isWarn && <AlertTriangle className="w-4 h-4 text-amber-400" />}
                    {isSec && <ShieldAlert className="w-4 h-4 text-purple-400" />}
                    <span
                      className={`font-mono font-bold text-[11px] ${
                        isCrit ? 'text-red-400' : isWarn ? 'text-amber-400' : 'text-purple-400'
                      }`}
                    >
                      {f.rule_id}
                    </span>
                  </div>

                  {f.service && (
                    <button
                      onClick={() => onSelectService(f.service!)}
                      className="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 font-mono text-[10px] transition-colors"
                      title="Focus in Graph"
                    >
                      {f.service}
                    </button>
                  )}
                </div>

                {/* Message */}
                <p className="text-slate-200 font-medium leading-relaxed">{f.message}</p>

                {/* Detail */}
                {f.detail && <p className="text-slate-400 text-[11px] leading-relaxed">{f.detail}</p>}

                {/* Recommendation */}
                <div className="bg-slate-900/60 p-2.5 rounded-lg border border-slate-800/80">
                  <span className="font-semibold text-blue-300 block mb-1">Recommendation</span>
                  <span className="text-slate-300 text-[11px] leading-relaxed">
                    {f.recommendation}
                  </span>
                </div>

                {/* Suggested Override & 1-Click Fix */}
                {f.suggested_override && (
                  <div className="space-y-1.5 pt-1">
                    <div className="flex items-center justify-between text-[10px] text-slate-400">
                      <span>Suggested Override Snippet</span>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => handleCopyOverride(f.suggested_override!, idx)}
                          className="flex items-center gap-1 text-slate-400 hover:text-white transition-colors"
                        >
                          {copiedIdx === idx ? (
                            <>
                              <Check className="w-3 h-3 text-emerald-400" /> Copied
                            </>
                          ) : (
                            <>
                              <Copy className="w-3 h-3" /> Copy
                            </>
                          )}
                        </button>

                        <button
                          onClick={() => handleApply(f.suggested_override!, idx)}
                          disabled={applyingIdx === idx}
                          className="flex items-center gap-1 px-2 py-0.5 rounded bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-medium transition-colors"
                          title="Apply this snippet directly into docker-compose.override.yml"
                        >
                          {appliedIdx === idx ? (
                            <>
                              <Check className="w-3 h-3 text-emerald-300" /> Applied!
                            </>
                          ) : applyingIdx === idx ? (
                            <span>Applying...</span>
                          ) : (
                            <>
                              <Wrench className="w-3 h-3" /> Apply Fix
                            </>
                          )}
                        </button>
                      </div>
                    </div>
                    <pre className="p-2.5 rounded bg-black/60 border border-slate-800 text-[11px] font-mono text-emerald-300 overflow-x-auto">
                      <code>{f.suggested_override}</code>
                    </pre>
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </aside>
  );
};
