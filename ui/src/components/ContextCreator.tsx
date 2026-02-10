import React from 'react';
import { EvalContext, Checkpoint } from '../workspace/types';
import { AlertTriangle, RefreshCw } from 'lucide-react';
import { EvalContextDisplay } from './EvalContextDisplay';

interface ContextCreatorProps {
  gtText: string;
  setGtText: (text: string) => void;
  gtAtGeneration: string | null;
  context?: EvalContext;
  loading: boolean;
  error: string | null;
  onGenerate: () => void;
  onPrimaryAction: () => void;
  initialContext?: EvalContext;
  onCancel: () => void;
  disablePrimary?: boolean;
  onCheckpointClick?: (checkpoint: Checkpoint) => void;
}

export const ContextCreator: React.FC<ContextCreatorProps> = ({
  gtText,
  setGtText,
  gtAtGeneration,
  context,
  loading,
  error,
  onGenerate,
  onPrimaryAction,
  initialContext,
  onCancel,
  disablePrimary,
  onCheckpointClick,
}) => {
  // Context is stale if GT changed since generation
  const isStale = context && gtAtGeneration !== null && gtText !== gtAtGeneration;
  // If we have initial context (editing), we can save only if context exists and not stale.
  // disablePrimary coming from parent handles the "no changes" check for existing context.
  // For new context creation (!initialContext), disablePrimary is false, but we need context + non-stale.
  const canSave = context && !isStale && !loading;

  return (
    <>
      {/* Main Content - flex container for equal GT and Checkpoints space */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Ground Truth Section - Fixed min height, max height constraint to prevent overflow */}
        <div className="px-6 py-4 flex flex-col bg-slate-100 border-b border-slate-100 shrink-0 min-h-[180px] max-h-[35vh] overflow-y-auto">
          <div className="flex items-center gap-2 mb-2 shrink-0">
            <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">Ground Truth</label>
            {error && <span className="ml-auto text-red-500 text-xs">{error}</span>}
          </div>
          <textarea
            className={`flex-1 border-none rounded-lg p-4 text-slate-700 font-mono text-sm resize-none focus:outline-none focus:ring-1 focus:ring-slate-200 min-h-[5rem] transition-colors ${loading ? 'bg-slate-50 opacity-75 cursor-wait' : 'bg-white'}`}
            value={gtText}
            onChange={(e) => setGtText(e.target.value)}
            placeholder="Enter Ground Truth..."
            readOnly={loading}
          />
          {!loading && context?.meta.questionable_gt && (
            <div className="mt-2 bg-amber-50 border border-amber-200 rounded-lg p-4 shrink-0">
              <div className="flex items-center gap-1.5 mb-1">
                <AlertTriangle size={12} className="text-amber-600 shrink-0" />
                <span className="text-xs font-bold text-amber-800 uppercase tracking-tight">GT Quality Alert</span>
              </div>
              <p className="text-xs text-amber-900/80 leading-relaxed font-medium">
                {context.meta.questionable_reason}
              </p>
            </div>
          )}
        </div>

        {/* Eval Context Section - Takes remaining space */}
        <div className="px-6 py-4 flex-1 min-h-0 flex flex-col">
          <div className="flex items-center gap-3 mb-3 shrink-0">
            <span className="text-xs font-bold text-slate-400 uppercase tracking-wider">Eval Context</span>

            {/* Stale Alert - Compact Badge */}
            {!loading && isStale && (
              <div className="flex items-center gap-1.5 px-2.5 py-1 bg-amber-50 border border-amber-200 rounded text-amber-800 animate-pulse">
                <AlertTriangle size={12} className="text-amber-600" />
                <span className="text-[10px] font-bold uppercase tracking-wide">Stale</span>
              </div>
            )}
            <button
              onClick={onGenerate}
              disabled={loading}
              className="ml-auto px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-white rounded-lg text-xs font-bold transition-colors disabled:opacity-50 flex items-center gap-1.5"
            >
              {loading ? (
                <>
                  <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                  Generating...
                </>
              ) : (
                <>
                  <RefreshCw size={12} />
                  {context ? 'Regenerate' : 'Generate'}
                </>
              )}
            </button>
          </div>

          {loading ? (
            <div className="flex-1 flex flex-col min-h-0 animate-pulse">
              {/* Skeleton Business Goal */}
              <div className="h-4 bg-slate-200 rounded w-3/4 mb-2 shrink-0"></div>
              <div className="h-4 bg-slate-200 rounded w-1/2 mb-4 shrink-0"></div>

              {/* Skeleton Header */}
              <div className="h-6 bg-slate-200 rounded w-32 mb-4 shrink-0"></div>

              {/* Skeleton Checkpoints */}
              <div className="flex flex-wrap gap-3">
                {[...Array(12)].map((_, i) => (
                  <div key={i} className="h-6 bg-slate-200 rounded-md w-24"></div>
                ))}
              </div>
            </div>
          ) : context ? (
            <EvalContextDisplay
              context={context}
              enableAudioRealityToggle={true}
              showWeightInBadge={true}
              onCheckpointClick={onCheckpointClick}
              className="flex-1 min-h-0"
            />
          ) : (
            <div className="flex-1 flex items-center justify-center text-slate-300 italic text-sm">
              No context generated yet. Click Generate to create.
            </div>
          )}
        </div>
      </div>

      {/* Footer */}
      <div className="px-6 py-3 border-t border-slate-200 bg-slate-50 flex items-center justify-end gap-3 shrink-0">
        <button onClick={onCancel} className="px-4 py-2 text-sm text-slate-600 hover:text-slate-800 font-medium transition-colors">
          Cancel
        </button>
        <button
          onClick={onPrimaryAction}
          disabled={!canSave || disablePrimary}
          className="px-5 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-lg font-bold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {initialContext ? 'Review Changes' : 'Save Context'}
        </button>
      </div>
    </>
  );
};
