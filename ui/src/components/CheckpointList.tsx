import React from 'react';
import { Checkpoint } from '../types';
import { RichTooltip } from './RichTooltip';

// --- CheckpointItem ---

interface CheckpointItemProps {
  checkpoint: Checkpoint;
  maxWeight: number;
  displayText?: React.ReactNode;
  showWeightInBadge?: boolean;
  isNew?: boolean;
}

export const CheckpointItem: React.FC<CheckpointItemProps> = ({
  checkpoint: cp,
  maxWeight,
  displayText,
  showWeightInBadge = true,
  isNew = false,
}) => {
  const opacity = maxWeight > 0 ? 0.5 + 0.5 * (cp.weight / maxWeight) : 1;

  // Render the badge (ID + optional weight)
  const renderBadge = () => (
    <span className={`inline-flex items-center justify-center px-1.5 py-0.5 rounded text-[9px] font-black mr-1 align-middle border gap-1 min-w-[1.5rem] ${cp.tier === 1 ? 'bg-red-50 text-red-600 border-red-200' :
      cp.tier === 2 ? 'bg-amber-50 text-amber-600 border-amber-200' :
        'bg-slate-50 text-slate-500 border-slate-200'
      }`}>
      {cp.id}
      {showWeightInBadge && (
        <span className="opacity-60 font-mono">
          {Math.round(cp.weight * 100)}%
        </span>
      )}
    </span>
  );

  return (
    <RichTooltip
      key={cp.id}
      trigger={
        <span
          style={{ opacity }}
          className={`inline rounded cursor-help transition-all duration-200 group ${cp.tier === 1 ? 'underline decoration-[3px] underline-offset-4 decoration-red-400' :
            cp.tier === 2 ? 'underline decoration-[3px] underline-offset-4 decoration-amber-400' :
              ''
            } hover:bg-slate-100 ${isNew ? 'bg-green-50/50 decoration-green-300' : ''}`}
        >
          {renderBadge()}
          <span className="text-sm leading-snug">
            {displayText || cp.text_segment}
          </span>
        </span>
      }
    >
      <div className="p-4 space-y-3 text-left max-w-xs">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-2 min-w-0">
            <span className={`shrink-0 px-1.5 py-0.5 rounded text-[10px] font-black border ${cp.tier === 1 ? 'bg-red-50 text-red-700 border-red-100' :
              cp.tier === 2 ? 'bg-amber-50 text-amber-700 border-amber-100' :
                'bg-slate-50 text-slate-700 border-slate-100'
              }`}>
              {cp.id}
            </span>
            <span className="text-[10px] font-bold text-slate-400 uppercase tracking-wider truncate">Tier {cp.tier}</span>
          </div>
          <span className="shrink-0 text-[11px] font-mono font-black text-slate-800 bg-slate-100 px-2 py-0.5 rounded-full">
            {Math.round(cp.weight * 100)}%
          </span>
        </div>
        {cp.rationale && (
          <div className="space-y-1">
            <span className="text-[9px] font-black text-slate-400 uppercase tracking-widest block text-left">Rationale</span>
            <p className="text-xs text-slate-600 leading-relaxed font-medium italic text-left">
              {cp.rationale}
            </p>
          </div>
        )}
      </div>
    </RichTooltip>
  );
};

// --- CheckpointList ---

interface CheckpointListProps {
  checkpoints: Checkpoint[];
  showWeightInBadge?: boolean;
  className?: string;
  renderDisplayText?: (checkpoint: Checkpoint) => React.ReactNode | undefined;
}

export const CheckpointList: React.FC<CheckpointListProps> = ({
  checkpoints,
  showWeightInBadge = true,
  className = '',
  renderDisplayText,
}) => {
  if (!checkpoints.length) return null;

  const maxWeight = Math.max(...checkpoints.map(cp => cp.weight));

  return (
    <div className={`flex flex-wrap items-center gap-x-3 gap-y-2 text-sm text-slate-700 leading-relaxed ${className}`}>
      {checkpoints.map((cp) => (
        <CheckpointItem
          key={cp.id}
          checkpoint={cp}
          maxWeight={maxWeight}
          showWeightInBadge={showWeightInBadge}
          displayText={renderDisplayText?.(cp)}
        />
      ))}
    </div>
  );
};
