import React, { useState, useMemo } from 'react';
import { EvalContext, Checkpoint } from '../workspace/types';
import { Copy } from 'lucide-react';
import { smartDiff } from '../diffUtils';
import { CheckpointList } from './CheckpointList';

interface EvalContextDisplayProps {
  context: EvalContext;
  enableAudioRealityToggle?: boolean;
  showWeightInBadge?: boolean;
  onCheckpointClick?: (checkpoint: Checkpoint) => void;
  className?: string;
}

export const EvalContextDisplay: React.FC<EvalContextDisplayProps> = ({
  context,
  enableAudioRealityToggle = false,
  showWeightInBadge = true,
  onCheckpointClick,
  className = '',
}) => {
  const [showAudioReality, setShowAudioReality] = useState(false);

  // Audio reality diff overlay
  const segmentDiffs = useMemo(() => {
    if (!showAudioReality || !context?.meta.audio_reality_inference) return null;
    return computeSegmentedDiff(context.checkpoints, context.meta.audio_reality_inference);
  }, [showAudioReality, context]);

  return (
    <div className={`flex flex-col ${className}`}>
      {/* Business Goal */}
      <p className="text-sm text-slate-700 leading-relaxed italic opacity-80 mb-3 shrink-0">
        {context.meta.business_goal}
      </p>

      {/* Toggle & Copy Section */}
      {enableAudioRealityToggle && context.meta.audio_reality_inference && (
        <div className="flex items-center mb-4 shrink-0">
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
            }}
            className="ml-2 p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-lg transition-colors"
            title={showAudioReality ? "Copy Audio Reality Inference" : "Copy Ground Truth"}
          >
            <Copy size={14} />
          </button>
        </div>
      )}

      {/* Checkpoints List */}
      <div className="flex-1 min-h-0 overflow-y-auto">
        <CheckpointList
          checkpoints={context.checkpoints}
          showWeightInBadge={showWeightInBadge}
          className="pb-2"
          renderDisplayText={(cp) => segmentDiffs?.has(cp.id) ? renderDiffParts(segmentDiffs.get(cp.id)!) : undefined}
          onCheckpointClick={onCheckpointClick}
        />
      </div>
    </div>
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

    // Move leading insertions from current to previous
    while (currentParts.length > 0 && currentParts[0].added) {
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
