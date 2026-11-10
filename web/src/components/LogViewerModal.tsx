import React, { useState, useEffect, useRef } from 'react';
import { LogLine } from '../types';
import { X, Search, Terminal, ArrowDown, Copy, Check, Trash2 } from 'lucide-react';

interface LogViewerModalProps {
  service: string | null;
  onClose: () => void;
}

export const LogViewerModal: React.FC<LogViewerModalProps> = ({ service, onClose }) => {
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [search, setSearch] = useState('');
  const [streamFilter, setStreamFilter] = useState<'all' | 'stdout' | 'stderr'>('all');
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!service) return;

    setLogs([]);
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/logs?service=${encodeURIComponent(
      service
    )}`;
    const ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
      try {
        const line: LogLine = JSON.parse(event.data);
        setLogs((prev) => [...prev.slice(-1000), line]);
      } catch (e) {
        console.error('Failed to parse log line', e);
      }
    };

    return () => {
      ws.close();
    };
  }, [service]);

  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoScroll]);

  if (!service) return null;

  const filteredLogs = logs.filter((l) => {
    if (streamFilter !== 'all' && l.stream !== streamFilter) return false;
    if (search && !l.message.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const handleCopy = () => {
    const text = filteredLogs.map((l) => `[${l.timestamp}] [${l.stream}] ${l.message}`).join('\n');
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-6">
      <div className="w-full max-w-4xl h-[650px] bg-[#0c1017] border border-slate-800 rounded-xl shadow-2xl flex flex-col overflow-hidden">
        {/* Header */}
        <div className="p-3.5 border-b border-slate-800/80 bg-[#111827] flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Terminal className="w-4 h-4 text-blue-400" />
            <span className="font-semibold text-sm text-slate-100 font-mono">
              service:{service}
            </span>
            <span className="text-xs text-slate-400">({logs.length} lines)</span>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={() => setAutoScroll(!autoScroll)}
              className={`p-1.5 rounded text-xs flex items-center gap-1 transition-colors ${
                autoScroll ? 'bg-blue-600/20 text-blue-400 border border-blue-500/40' : 'text-slate-400 hover:text-white'
              }`}
              title="Toggle Auto-Scroll"
            >
              <ArrowDown className="w-3.5 h-3.5" /> Auto-scroll
            </button>
            <button
              onClick={handleCopy}
              className="p-1.5 rounded text-xs text-slate-400 hover:text-white hover:bg-slate-800 transition-colors flex items-center gap-1"
              title="Copy visible logs"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
            <button
              onClick={() => setLogs([])}
              className="p-1.5 rounded text-xs text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
              title="Clear terminal buffer"
            >
              <Trash2 className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={onClose}
              className="p-1.5 rounded text-slate-400 hover:text-white hover:bg-slate-800 transition-colors ml-2"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Search & Filter Bar */}
        <div className="p-2.5 border-b border-slate-800/60 bg-[#0d131f] flex items-center gap-3 text-xs">
          <div className="relative flex-1">
            <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Filter logs by keyword..."
              className="w-full pl-8 pr-3 py-1 bg-slate-900/80 border border-slate-800 rounded-md text-slate-200 placeholder:text-slate-500 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div className="flex rounded-md overflow-hidden border border-slate-800">
            <button
              onClick={() => setStreamFilter('all')}
              className={`px-2.5 py-1 ${streamFilter === 'all' ? 'bg-slate-800 text-white' : 'text-slate-400'}`}
            >
              All
            </button>
            <button
              onClick={() => setStreamFilter('stdout')}
              className={`px-2.5 py-1 ${streamFilter === 'stdout' ? 'bg-slate-800 text-white' : 'text-slate-400'}`}
            >
              stdout
            </button>
            <button
              onClick={() => setStreamFilter('stderr')}
              className={`px-2.5 py-1 ${streamFilter === 'stderr' ? 'bg-slate-800 text-white' : 'text-slate-400'}`}
            >
              stderr
            </button>
          </div>
        </div>

        {/* Logs Output Area */}
        <div className="flex-1 p-4 font-mono text-[11px] leading-relaxed overflow-y-auto space-y-1 select-text bg-[#030712]">
          {filteredLogs.length === 0 ? (
            <div className="text-slate-500 italic py-8 text-center">
              No log frames received yet or no lines match filter.
            </div>
          ) : (
            filteredLogs.map((l, idx) => (
              <div key={idx} className="flex items-start gap-2 hover:bg-slate-900/40 px-1.5 py-0.5 rounded">
                <span className="text-slate-500 select-none text-[10px] w-24 flex-shrink-0">
                  {new Date(l.timestamp).toLocaleTimeString()}
                </span>
                <span
                  className={`text-[9px] uppercase px-1 rounded select-none flex-shrink-0 font-bold ${
                    l.stream === 'stderr' ? 'bg-amber-950/60 text-amber-400' : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  {l.stream}
                </span>
                <span
                  className={`break-all ${
                    l.stream === 'stderr' ? 'text-amber-200/90' : 'text-slate-200'
                  }`}
                >
                  {l.message}
                </span>
              </div>
            ))
          )}
          <div ref={logsEndRef} />
        </div>
      </div>
    </div>
  );
};
