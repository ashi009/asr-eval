import { useState } from 'react';
import { Copy, Check, Minus, AlertTriangle } from 'lucide-react';
import { getASRProviderConfig } from '../config';
import { Case } from '../workspace/types';
import { renderDiff } from './DiffRenderer';
import { isResultStale } from '../utils/evalUtils';
import { RichTooltip } from './RichTooltip';

interface EvalReportViewProps {
  kase: Case;
  selectedProviders: Record<string, boolean>;
  onToggleProvider: (provider: string) => void;
  onSelectAll: () => void;
  onDeselectAll: () => void;
  onSelectDefault: () => void;
  getDefaultSelection: () => Record<string, boolean>;
  isProcessing?: boolean;
}

export function EvalReportView({ kase, selectedProviders, onToggleProvider, onSelectAll, onDeselectAll, onSelectDefault, getDefaultSelection, isProcessing }: EvalReportViewProps) {
  const evalResults = kase.report_v2?.evaluations || {};
  const hasAI = Object.keys(evalResults).length > 0;

  const providers = Array.from(new Set([
    ...Object.keys(evalResults),
    ...Object.keys(kase.transcripts || {})
  ]));
  const sortedPerformers = Object.entries(evalResults).sort((a, b) => (b[1]?.metrics?.Q_score ?? 0) - (a[1]?.metrics?.Q_score ?? 0));

  const [diffModes, setDiffModes] = useState<Record<string, 'eval' | 'drift' | 'gap'>>({});
  const [sortBy, setSortBy] = useState<'score' | 'name'>('score');

  const sortedProviders = [...providers].sort((a, b) => {
    if (sortBy === 'score') {
      const scoreA = evalResults[a]?.metrics?.Q_score ?? -1;
      const scoreB = evalResults[b]?.metrics?.Q_score ?? -1;
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
    <div className="flex flex-col h-full">
      <div className="bg-white px-8 flex flex-col shrink-0">
        {/* Persistent Axis Separator */}
        <div className="relative h-px bg-slate-500 z-20 -top-px">
          {hasAI && !isProcessing && (
            <div className="absolute inset-0">
              <div className="relative w-full h-full">
                {/* Score Dots with Tooltips */}
                {sortedPerformers.map(([p, res]) => {
                  const score = res?.metrics?.Q_score ?? 0;
                  const config = getASRProviderConfig(p);
                  return (
                    <RichTooltip
                      key={p}
                      className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 cursor-pointer group"
                      style={{ left: `${score}%` }}
                      trigger={
                        <div className={`w-3 h-3 rounded-full ${config.color.dot} border-2 border-white shadow-sm hover:scale-125 transition-transform`} />
                      }
                    >
                      <div className="px-3 py-2 text-slate-700">
                        <div className="flex items-center justify-between gap-4">
                          <span className="text-[11px] font-bold text-slate-600 uppercase tracking-wider">{config.name}</span>
                          <span className="text-[12px] font-mono font-black text-slate-800 bg-slate-100 px-2 py-0.5 rounded-full">{Math.round(score)}</span>
                        </div>
                      </div>
                    </RichTooltip>
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
          const score = aiRes?.metrics?.Q_score ?? null; // Use null if metrics missing
          const config = getASRProviderConfig(p);
          const { color, name } = config;
          const isSelected = !!selectedProviders?.[p];
          const stale = isStale(p);
          const mode = diffModes[p] || 'eval';
          const isLoading = isProcessing && isSelected;

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
                {isLoading ? (
                  // Loading Skeleton for Score
                  <div className="mt-1 animate-pulse">
                    <div className="h-8 w-12 bg-slate-200 rounded mb-1"></div>
                    <div className="h-3 w-16 bg-slate-100 rounded"></div>
                  </div>
                ) : stale ? (
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
                  <>
                    <div className={`text-3xl font-bold mt-1 ${scoreColorClass}`}>
                      {score !== null ? score : <span className="text-slate-300 text-lg">—</span>}
                    </div>
                    {aiRes?.metrics?.S_score !== undefined && aiRes?.metrics?.P_score !== undefined && (
                      <div className="text-[10px] text-slate-400 font-medium mt-0.5 font-mono">
                        S{Math.round(aiRes.metrics.S_score * 100)} P{Math.round(aiRes.metrics.P_score * 100)}
                      </div>
                    )}
                  </>
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
                    navigator.clipboard.writeText(kase.transcripts?.[p] || "");
                  }}
                  title="Copy original transcript"
                >
                  <Copy size={14} />
                </button>
              </div>

              {/* Column 3: Analysis */}
              <div className="text-xs text-slate-400">
                {isLoading ? (
                  // Loading Skeleton for Analysis
                  <div className="space-y-2 animate-pulse">
                    <div className="h-4 bg-slate-200 rounded w-full"></div>
                    <div className="h-4 bg-slate-200 rounded w-5/6"></div>
                    <div className="h-4 bg-slate-200 rounded w-4/6"></div>
                  </div>
                ) : aiRes?.summary ? (
                  <ul className="space-y-1.5">
                    {aiRes.summary.map((point, i) => {
                      // Parse checkpoint references like S1, S2, etc. and render as badges
                      const parts = point.split(/(S\d+)/g);
                      return (
                        <li key={i} className="leading-snug">
                          {parts.map((part, j) => {
                            if (/^S\d+$/.test(part)) {
                              const checkpoint = kase.eval_context?.checkpoints.find(cp => cp.id === part);
                              const result = aiRes.checkpoint_results?.[part];
                              const tier = checkpoint?.tier ?? 3;
                              const badgeClass = tier === 1
                                ? 'bg-red-50 text-red-600 border-red-200'
                                : tier === 2
                                  ? 'bg-amber-50 text-amber-600 border-amber-200'
                                  : 'bg-slate-50 text-slate-600 border-slate-200';

                              return (
                                <RichTooltip
                                  key={j}
                                  className="inline-block mb-0.5"
                                  trigger={
                                    <span
                                      className={`inline-flex items-center justify-center px-1.5 py-0.5 rounded text-[9px] font-black mx-0.5 align-middle border cursor-help hover:bg-white transition-colors ${badgeClass}`}
                                    >
                                      {part}
                                    </span>
                                  }
                                >
                                  <div className="p-4 space-y-3.5 text-left w-72">
                                    {/* Header: Exact same as Context Detail */}
                                    <div className="flex items-center justify-between gap-4">
                                      <div className="flex items-center gap-2 min-w-0">
                                        <span className={`shrink-0 px-1.5 py-0.5 rounded text-[10px] font-black border ${tier === 1 ? 'bg-red-50 text-red-700 border-red-100' :
                                          tier === 2 ? 'bg-amber-50 text-amber-700 border-amber-100' :
                                            'bg-slate-50 text-slate-700 border-slate-100'
                                          }`}>
                                          {part}
                                        </span>
                                        <span className="text-[10px] font-bold text-slate-400 uppercase tracking-wider truncate">Tier {tier}</span>
                                      </div>
                                      <span className="shrink-0 text-[11px] font-mono font-black text-slate-800 bg-slate-100 px-2 py-0.5 rounded-full">
                                        {checkpoint ? Math.round(checkpoint.weight * 100) : 0}%
                                      </span>
                                    </div>

                                    {/* Segment Analysis (Diff) */}
                                    {checkpoint?.text_segment && (
                                      <div className="space-y-1">
                                        <span className="text-[9px] font-black text-slate-400 uppercase tracking-widest block">Original vs Detected</span>
                                        <div className="text-[11px] text-slate-700 leading-relaxed font-medium">
                                          {result?.detected ? renderDiff(checkpoint.text_segment, result.detected) : checkpoint.text_segment}
                                        </div>
                                      </div>
                                    )}

                                    {/* AI Detection Reason (with Status Badge as label) */}
                                    {result?.reason && (
                                      <div className="pt-2 space-y-1">
                                        {result?.status && (
                                          <span className={`inline-block text-[9px] font-black uppercase px-1.5 py-0.5 rounded-full border mb-0.5 ${result.status === 'Pass' ? 'bg-green-50 text-green-700 border-green-100' :
                                            result.status === 'Fail' ? 'bg-red-50 text-red-700 border-red-100' :
                                              'bg-amber-50 text-amber-700 border-amber-100'
                                            }`}>
                                            {result.status}
                                          </span>
                                        )}
                                        <p className="text-[11px] text-slate-600 leading-relaxed font-semibold">
                                          {result.reason}
                                        </p>
                                      </div>
                                    )}
                                  </div>
                                </RichTooltip>
                              );
                            }
                            return <span key={j}>{part}</span>;
                          })}
                        </li>
                      );
                    })}
                  </ul>
                ) : (
                  <span className="text-slate-300 text-lg">—</span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
