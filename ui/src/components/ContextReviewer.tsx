import React from 'react';
import { EvalContext, Checkpoint } from '../types';
import { smartDiff } from '../diffUtils';
import { CheckpointList } from './CheckpointList';

interface ContextReviewerProps {
  oldContext: EvalContext;
  newContext: EvalContext;
  oldGT: string;
  newGT: string;
  onBack: () => void;
  onSave: () => void;
  onCancel: () => void;
  onCheckpointClick?: (checkpoint: Checkpoint) => void;
}

export const ContextReviewer: React.FC<ContextReviewerProps> = ({
  oldContext,
  newContext,
  oldGT,
  newGT,
  onBack,
  onSave,
  onCancel,
  onCheckpointClick,
}) => {
  return (
    <>
      {/* Content */}
      <div className="flex-1 overflow-y-auto p-8 space-y-8">

        {/* Section 1: Diff Views (GT & Audio Reality) - Minimalist */}
        <div className="space-y-6">
          <div>
            <div className="text-xs font-bold text-slate-400 uppercase mb-2">Ground Truth</div>
            <div className="text-sm font-mono text-slate-700 leading-relaxed">
              {oldGT !== newGT ? renderSimpleDiff(oldGT, newGT) : newGT}
            </div>
          </div>

          <div>
            <div className="text-xs font-bold text-slate-400 uppercase mb-2">Audio Reality Inference</div>
            <div className="text-sm italic text-slate-600 leading-relaxed">
              {(oldContext.meta.audio_reality_inference || '') !== (newContext.meta.audio_reality_inference || '')
                ? renderSimpleDiff(oldContext.meta.audio_reality_inference || '', newContext.meta.audio_reality_inference || '')
                : (newContext.meta.audio_reality_inference || '—')
              }
            </div>
          </div>
        </div>

        <div className="h-px bg-slate-200" />

        {/* Section 2: Side-by-Side Comparison Grid */}
        <div className="grid grid-cols-2 gap-x-12 gap-y-6">
          {/* Headers */}
          <div className="text-xs font-bold text-slate-500 uppercase tracking-wider border-b border-slate-200 pb-2">Existing Eval Context</div>
          <div className="text-xs font-bold text-blue-600 uppercase tracking-wider border-b border-blue-100 pb-2">New Eval Context</div>

          {/* Business Goals */}
          <div>
            <div className="text-xs font-bold text-slate-400 uppercase mb-1">Business Goal</div>
            <p className="text-sm text-slate-600 italic leading-relaxed">{oldContext.meta.business_goal}</p>
          </div>
          <div>
            <div className="text-xs font-bold text-slate-400 uppercase mb-1">Business Goal</div>
            <p className="text-sm text-slate-600 italic leading-relaxed">{newContext.meta.business_goal}</p>
          </div>

          {/* Checkpoints - Each side in its own container */}
          <div>
            <div className="text-xs font-bold text-slate-400 uppercase mb-2">Checkpoints</div>
            <CheckpointList checkpoints={oldContext.checkpoints} showWeightInBadge={true} onCheckpointClick={onCheckpointClick} />
          </div>
          <div>
            <div className="text-xs font-bold text-slate-400 uppercase mb-2">Checkpoints</div>
            <CheckpointList checkpoints={newContext.checkpoints} showWeightInBadge={true} onCheckpointClick={onCheckpointClick} />
          </div>
        </div>

      </div>

      {/* Footer */}
      <div className="px-6 py-3 border-t border-slate-200 bg-slate-50 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-4">
          <button onClick={onBack} className="text-sm text-slate-500 hover:text-slate-800 font-medium transition-colors">
            ← Back to Edit
          </button>
          <div className="h-4 w-px bg-slate-300" />
          <span className="text-xs text-amber-700 font-medium flex items-center gap-1.5 bg-amber-50 px-2 py-1 rounded border border-amber-200">
            ⚠️ Saving will reset all existing evaluation results
          </span>
        </div>
        <div className="flex items-center gap-3">
          <button onClick={onCancel} className="px-4 py-2 text-sm text-slate-600 hover:text-slate-800 font-medium transition-colors">
            Cancel
          </button>
          <button
            onClick={onSave}
            className="px-5 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-lg font-bold transition-colors"
          >
            Save & Reset Results
          </button>
        </div>
      </div>
    </>
  );
};

function renderSimpleDiff(oldText: string, newText: string): React.ReactNode {
  const diffs = smartDiff(oldText, newText, true);
  return (
    <span>
      {diffs.map((part, i) => {
        const text = part.value;
        if (part.added) return <span key={i} className="bg-green-100 text-green-700">{text}</span>;
        if (part.removed) return <span key={i} className="bg-red-100 text-red-400 line-through">{text}</span>;
        return <span key={i}>{text}</span>;
      })}
    </span>
  );
}
