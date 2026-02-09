import React, { useState, useMemo } from 'react';
import { ContextResponse, Checkpoint } from '../types';
import { AlertTriangle, RefreshCw, Copy } from 'lucide-react';
import { smartDiff } from '../diffUtils';
import { CheckpointList } from './CheckpointList';

interface ContextCreatorProps {
  gtText: string;
  setGtText: (text: string) => void;
  gtAtGeneration: string | null;
  context?: ContextResponse;
  loading: boolean;
  error: string | null;
  onGenerate: () => void;
  onPrimaryAction: () => void;
  initialContext?: ContextResponse;
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
  const [showAudioReality, setShowAudioReality] = useState(false);

  // Audio reality diff overlay
  const segmentDiffs = useMemo(() => {
    if (!showAudioReality || !context?.meta.audio_reality_inference) return null;
    return computeSegmentedDiff(context.checkpoints, context.meta.audio_reality_inference);
  }, [showAudioReality, context]);

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
            <div className="flex-1 flex flex-col min-h-0">
              {/* Business Goal */}
              <p className="text-sm text-slate-600 leading-relaxed italic mb-3 shrink-0">
                {context.meta.business_goal}
              </p>

              {/* Checkpoints List - matching CaseDetail style with tooltips */}
              <div className="flex-1 min-h-0 overflow-y-auto">
                {/* Toggle for audio reality */}
                {context.meta.audio_reality_inference && (
                  <div className="flex items-center mb-4">
                    <div className="flex items-center bg-slate-100 rounded-full p-0.5">
                      <button
                        onClick={() => setShowAudioReality(false)}
                        className={`text-[10px] font-medium px-3 py-1 rounded-full transition-colors ${!showAudioReality
                          ? 'bg-white text-slate-700 shadow-sm'
                          : 'text-slate-500 hover:text-slate-700'
                          }`}
                      >
                        Ground Truth
                      </button>
                      <button
                        onClick={() => setShowAudioReality(true)}
                        className={`text-[10px] font-medium px-3 py-1 rounded-full transition-colors ${showAudioReality
                          ? 'bg-white text-slate-700 shadow-sm'
                          : 'text-slate-500 hover:text-slate-700'
                          }`}
                      >
                        Audio Reality Inference
                      </button>
                    </div>

                    <button
                      onClick={() => {
                        const text = showAudioReality
                          ? context.meta.audio_reality_inference
                          : context.meta.ground_truth;
                        navigator.clipboard.writeText(text);
                        // Brief success state could be added here if desired, but user just asked for the button
                        // To keep it simple and consistent with other copy buttons that just do it:
                      }}
                      className="ml-2 p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-lg transition-colors"
                      title={showAudioReality ? "Copy Audio Reality Inference" : "Copy Ground Truth"}
                    >
                      <Copy size={14} />
                    </button>
                  </div>
                )}

                <CheckpointList
                  checkpoints={context.checkpoints}
                  showWeightInBadge={true}
                  className="pb-4"
                  renderDisplayText={(cp) => segmentDiffs?.has(cp.id) ? renderDiffParts(segmentDiffs.get(cp.id)!) : undefined}
                  onCheckpointClick={onCheckpointClick}
                />
              </div>
            </div>
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

// --- Helper functions for audio reality diff ---

interface SegmentDiffPart {
  value: string;
  added?: boolean;
  removed?: boolean;
}

function computeSegmentedDiff(
  checkpoints: Checkpoint[],
  audioReality: string
): Map<string, SegmentDiffPart[]> {
  // Use a special separator character to guide segmentation
  const SEP = '\u0000';
  const joinedSegments = checkpoints.map(cp => cp.text_segment).join(SEP);
  // smartDiff relies on diffWords which might split by words.
  // We need to ensure the separator text is treated uniquely if possible,
  // but diffWords generally works on whitespace.
  // However, since we are doing character-based diff essentially (or word based),
  // if SEP is distinct it should appear in the diff.
  const diffs = smartDiff(joinedSegments, audioReality, true);

  const result = new Map<string, SegmentDiffPart[]>();
  checkpoints.forEach(cp => result.set(cp.id, []));

  let cpIndex = 0;

  for (const part of diffs) {
    if (part.added) {
      if (cpIndex < checkpoints.length) {
        result.get(checkpoints[cpIndex].id)!.push({ value: part.value, added: true });
      }
    } else {
      // Both removed and unchanged parts may contain the separator
      // Note: diffWords might group SEP with surrounding text if no spaces.
      // But \u0000 is non-word usually.
      const segments = part.value.split(SEP);
      segments.forEach((seg, i) => {
        if (i > 0) {
          // Separator crossed
          if (cpIndex < checkpoints.length - 1) cpIndex++;
        }
        if (seg) {
          if (cpIndex < checkpoints.length) {
            result.get(checkpoints[cpIndex].id)!.push({
              value: seg,
              removed: part.removed
            });
          }
        }
      });
    }
  }

  // Post-processing: Bind leading deletions to the previous segment
  for (let i = 1; i < checkpoints.length; i++) {
    const currentId = checkpoints[i].id;
    const prevId = checkpoints[i - 1].id;
    const currentParts = result.get(currentId)!;
    const prevParts = result.get(prevId)!;

    // Move leading deletions AND insertions from current to previous
    while (currentParts.length > 0 && (currentParts[0].removed || currentParts[0].added)) {
      prevParts.push(currentParts.shift()!);
    }
  }
  return result;
}

function renderDiffParts(parts: SegmentDiffPart[]): React.ReactNode {
  return (
    <span>
      {parts.map((part, i) => {
        if (part.added) return <span key={i} className="text-green-600 font-medium">{part.value}</span>;
        if (part.removed) return <span key={i} className="text-red-400 line-through">{part.value}</span>;
        return <span key={i}>{part.value}</span>;
      })}
    </span>
  );
}
