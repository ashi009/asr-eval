import { useState } from 'react';
import { Copy, Check, Minus, AlertTriangle } from 'lucide-react';
import { getASRProviderConfig } from '../config';
import { Case } from '../types';
import { renderDiff } from './DiffRenderer';
import { isResultStale } from '../utils/evalUtils';

interface EvalReportViewProps {
  kase: Case;
  selectedProviders: Record<string, boolean>;
  onToggleProvider: (provider: string) => void;
  onSelectAll: () => void;
  onDeselectAll: () => void;
  onSelectDefault: () => void;
  getDefaultSelection: () => Record<string, boolean>;
}

export function EvalReportView({ kase, selectedProviders, onToggleProvider, onSelectAll, onDeselectAll, onSelectDefault, getDefaultSelection }: EvalReportViewProps) {
  const evalResults = kase.eval_report?.eval_results || {};
  const hasAI = Object.keys(evalResults).length > 0;

  const providers = Array.from(new Set([
    ...Object.keys(evalResults),
    ...Object.keys(kase.transcripts || {})
  ]));
  const sortedPerformers = Object.entries(evalResults).sort((a, b) => b[1].score - a[1].score);

  const [diffModes, setDiffModes] = useState<Record<string, 'eval' | 'drift' | 'gap'>>({});
  const [sortBy, setSortBy] = useState<'score' | 'name'>('score');

  const sortedProviders = [...providers].sort((a, b) => {
    if (sortBy === 'score') {
      const scoreA = evalResults[a]?.score ?? -1;
      const scoreB = evalResults[b]?.score ?? -1;
      if (scoreB !== scoreA) return scoreB - scoreA;
      return getASRProviderConfig(a).name.localeCompare(getASRProviderConfig(b).name);
    } else {
      return getASRProviderConfig(a).name.localeCompare(getASRProviderConfig(b).name);
    }
  });

  const selectedCount = providers.filter(p => selectedProviders?.[p]).length;
  const selectionState = selectedCount === 0 ? 'none' : selectedCount === providers.length ? 'all' : 'partial';

  const defaultSelection = getDefaultSelection ? getDefaultSelection() : {};
  const defaultSelectedCount = providers.filter(p => defaultSelection[p]).length;

  const handleHeaderCheckboxClick = () => {
    if (selectionState === 'none') {
      if (defaultSelectedCount > 0 && onSelectDefault) {
        onSelectDefault();
      } else {
        onSelectAll();
      }
    } else if (selectionState === 'partial') {
      onSelectAll();
    } else {
      onDeselectAll();
    }
  };

  const isStale = (provider: string) => {
    return isResultStale(kase.transcripts?.[provider], evalResults[provider]);
  };

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="bg-white px-8 flex flex-col shrink-0">
        {/* Case ID Header */}
        <div className="py-3 flex items-center justify-between">
          <div className="flex items-center gap-2 pl-8">
            <span className="text-sm font-mono font-medium text-slate-700 select-all">{kase.id}</span>
            <button
              onClick={() => {
                navigator.clipboard.writeText(kase.id);
              }}
              className="p-1 hover:bg-slate-100 rounded text-slate-400 hover:text-primary transition-colors"
              title="Copy Case ID"
            >
              <Copy size={12} />
            </button>
          </div>
        </div>

        {/* Persistent Axis Separator */}
        <div className="relative h-px bg-slate-200">
          {hasAI && (
            <div className="absolute inset-0">
              <div className="relative w-full h-full">
                {/* Score Dots with Tooltips */}
                {sortedPerformers.map(([p, res]) => {
                  const score = res.score * 100;
                  const config = getASRProviderConfig(p);
                  const isNearLeft = score < 15;
                  const isNearRight = score > 85;
                  let tooltipPosition = 'left-1/2 -translate-x-1/2';
                  if (isNearLeft) tooltipPosition = 'left-0';
                  else if (isNearRight) tooltipPosition = 'right-0';
                  return (
                    <div
                      key={p}
                      className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 cursor-pointer group"
                      style={{ left: `${score}%` }}
                    >
                      <div className={`w-3 h-3 rounded-full ${config.color.dot} border-2 border-white shadow-sm hover:scale-125 transition-transform`} />
                      <div className={`absolute bottom-full ${tooltipPosition} mb-2 px-2 py-1 bg-slate-800 text-white text-xs rounded whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-30`}>
                        {config.name}: {Math.round(score)}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        {/* Grid Header - border extends to container edges */}
        <div className="-mx-8 px-8 border-b border-slate-200">
          <div className="grid grid-cols-[32px_160px_1fr_240px] gap-0 py-3 text-[10px] font-bold uppercase tracking-wider text-slate-400 items-center">
            {/* Header checkbox - same style as row checkboxes */}
            <div className="flex justify-center">
              <button
                type="button"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  handleHeaderCheckboxClick();
                }}
                className="w-4 h-4 rounded-full flex items-center justify-center transition-all shrink-0 bg-white border-2 border-slate-400 shadow-sm cursor-pointer"
              >
                {selectionState === 'all' && <Check size={10} className="text-slate-500" strokeWidth={4} />}
                {selectionState === 'partial' && <Minus size={10} className="text-slate-500" strokeWidth={4} />}
              </button>
            </div>
            {/* Service / Score - clickable text to sort */}
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setSortBy('name')}
                className={`hover:text-slate-600 cursor-pointer ${sortBy === 'name' ? 'text-slate-700' : ''}`}
              >
                PROVIDER
              </button>
              <span>/</span>
              <button
                type="button"
                onClick={() => setSortBy('score')}
                className={`hover:text-slate-600 cursor-pointer ${sortBy === 'score' ? 'text-slate-700' : ''}`}
              >
                SCORE
              </button>
            </div>
            <div>TRANSCRIPT</div>
            <div>Analysis</div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto min-h-0 px-8">
        {sortedProviders.map((p) => {
          const aiRes = evalResults[p];
          const score = aiRes ? Math.round(aiRes.score * 100) : null;
          const config = getASRProviderConfig(p);
          const { color, name } = config;
          const isSelected = !!selectedProviders?.[p];
          const stale = isStale(p);
          const mode = diffModes[p] || 'eval';

          // Prepare Diff Texts
          const origin = aiRes?.transcript || ""; // Snapshot
          const current = kase.transcripts?.[p] || ""; // Current on disk
          const revised = aiRes?.revised_transcript || ""; // AI Revised

          let diffLeft = current;
          let diffRight = "";
          let showDiff = false;

          if (stale) {
            showDiff = true;
            if (mode === 'drift') {
              diffLeft = origin;
              diffRight = current;
            } else if (mode === 'gap') {
              diffLeft = current;
              diffRight = revised;
            } else {
              // mode === 'eval'
              diffLeft = origin;
              diffRight = revised;
            }
          } else if (aiRes) {
            // Normal case: compare Snapshot (which matches Current) vs Revised
            showDiff = true;
            diffLeft = origin;
            diffRight = revised;
          } else {
            // No Eval: just show current
            showDiff = false;
          }

          // Determine score color
          let scoreColorClass = 'text-red-500';
          if (score !== null) {
            if (score >= 90) scoreColorClass = 'text-green-600';
            else if (score >= 70) scoreColorClass = 'text-yellow-600';
            else if (score >= 50) scoreColorClass = 'text-orange-500';
          }

          return (
            <div
              id={`panel-${p}`}
              key={p}
              className={`grid grid-cols-[32px_160px_1fr_240px] gap-0 border-b border-slate-100 last:border-b-0 py-3 transition-colors items-start
                ${isSelected ? 'bg-slate-50' : 'hover:bg-slate-50/50'}
              `}
            >
              {/* Column 0: Round checkbox - always solid colored, white check when selected */}
              <div
                className="flex justify-center cursor-pointer"
                onClick={() => onToggleProvider && onToggleProvider(p)}
              >
                <div
                  className={`w-5 h-5 rounded-full flex items-center justify-center transition-all shrink-0 ${color.dot} border-2 border-white shadow-sm`}
                  style={{ marginTop: '0px' }}
                >
                  {isSelected && <Check size={12} className="text-white" strokeWidth={4} />}
                </div>
              </div>

              {/* Column 1: Service / Score - only title is clickable */}
              <div className="flex flex-col select-none">
                {/* Service name row - clickable */}
                <div
                  className="flex items-start gap-1.5 cursor-pointer"
                  onClick={() => onToggleProvider && onToggleProvider(p)}
                >
                  <span
                    className="text-xs font-bold uppercase tracking-wide text-slate-700 hover:text-slate-900"
                    style={{ lineHeight: '20px' }}
                  >
                    {name}
                  </span>
                </div>
                {/* Score - not clickable */}
                {stale ? (
                  <div className="mt-1 flex items-center gap-2" title="Transcript changed since evaluation">
                    <div className="text-3xl font-bold text-slate-300 line-through opacity-50">
                      {score}
                    </div>
                    <div className="flex items-center gap-1 text-amber-600 font-bold text-xs bg-amber-50 px-1.5 py-0.5 rounded border border-amber-200 uppercase whitespace-nowrap">
                      <AlertTriangle size={12} className="fill-amber-600 text-white" />
                      <span>Stale</span>
                    </div>
                  </div>
                ) : (
                  <div className={`text-3xl font-bold mt-1 ${scoreColorClass}`}>
                    {score !== null ? score : <span className="text-slate-300 text-lg">—</span>}
                  </div>
                )}
              </div>

              {/* Column 2: Transcript Diff - 20px line height to match */}
              <div
                className="text-sm text-slate-700 relative group pr-8"
                style={{ lineHeight: '20px' }}
              >
                {stale && (
                  <div className="mb-3 select-none">
                    <div className="inline-flex bg-slate-100 rounded-md p-1 gap-1">
                      <button
                        className={`px-3 py-1 text-xs uppercase font-bold rounded-md transition-all ${mode === 'eval' ? 'bg-white text-slate-800 shadow-sm ring-1 ring-black/5' : 'text-slate-500 hover:text-slate-700 hover:bg-slate-200/50'}`}
                        onClick={() => setDiffModes(prev => ({ ...prev, [p]: 'eval' }))}
                        title="Snapshot vs Revised (Original Eval)"
                      >
                        Origin vs Revised
                      </button>
                      <button
                        className={`px-3 py-1 text-xs uppercase font-bold rounded-md transition-all ${mode === 'drift' ? 'bg-white text-slate-800 shadow-sm ring-1 ring-black/5' : 'text-slate-500 hover:text-slate-700 hover:bg-slate-200/50'}`}
                        onClick={() => setDiffModes(prev => ({ ...prev, [p]: 'drift' }))}
                        title="Snapshot vs Current (What changed on disk)"
                      >
                        Origin vs New
                      </button>
                      <button
                        className={`px-3 py-1 text-xs uppercase font-bold rounded-md transition-all ${mode === 'gap' ? 'bg-white text-slate-800 shadow-sm ring-1 ring-black/5' : 'text-slate-500 hover:text-slate-700 hover:bg-slate-200/50'}`}
                        onClick={() => setDiffModes(prev => ({ ...prev, [p]: 'gap' }))}
                        title="Current vs Revised (How far is new from AI fix)"
                      >
                        New vs Revised
                      </button>
                    </div>
                  </div>
                )}

                {showDiff ? renderDiff(diffLeft, diffRight) : <span>{diffLeft}</span>}

                <button
                  className="absolute top-0 right-2 p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded transition-all opacity-0 group-hover:opacity-100"
                  onClick={(e) => {
                    e.stopPropagation();
                    navigator.clipboard.writeText(kase.transcripts[p] || "");
                  }}
                  title="Copy original transcript"
                >
                  <Copy size={14} />
                </button>
              </div>

              {/* Column 3: Analysis */}
              <div className="text-xs text-slate-400">
                {aiRes?.summary ? (
                  <ul className="space-y-1.5">
                    {aiRes.summary.map((point, i) => (
                      <li key={i} className="leading-snug">
                        {point}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <span className="text-slate-300 text-lg">—</span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div >
  );
};
