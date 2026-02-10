import React, { useState, useEffect } from 'react';
import { EvalContext, Checkpoint } from '../workspace/types';
import { useWorkspace } from '../workspace/context';
import { ContextCreator } from './ContextCreator';
import { ContextReviewer } from './ContextReviewer';
import { AudioPlayer, AudioPlayerHandle } from './AudioPlayer';
import { X } from 'lucide-react';

interface ContextManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
  caseId: string;
  initialGT: string;
  initialContext?: EvalContext;
  onSave: (context: EvalContext, gt: string) => void;
}

type ViewMode = 'EDITOR' | 'COMPARISON';

export const ContextManagerModal: React.FC<ContextManagerModalProps> = ({
  isOpen,
  onClose,
  caseId,
  initialGT,
  initialContext,
  onSave,
}) => {
  const [view, setView] = useState<ViewMode>('EDITOR');
  // Lifted state
  const [gtText, setGtText] = useState(initialGT);
  const [gtAtGeneration, setGtAtGeneration] = useState<string | null>(null);
  const [context, setContext] = useState<EvalContext | undefined>(initialContext);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const audioPlayerRef = React.useRef<AudioPlayerHandle>(null);

  const [abortController, setAbortController] = useState<AbortController | null>(null);

  useEffect(() => {
    if (isOpen) {
      // Reset state on open
      setGtText(initialGT);
      setGtAtGeneration(initialContext ? initialGT : null);
      setContext(initialContext);
      setError(null);
      setView('EDITOR');
    }
  }, [isOpen, initialGT, initialContext]);

  useEffect(() => {
    return () => {
      // Abort on close or unmount
      if (abortController) {
        abortController.abort();
      }
    };
  }, [abortController]);

  if (!isOpen) return null;

  const { generateContext, updateContext } = useWorkspace();

  // Handlers for Creator
  const handleGenerate = async () => {
    // Abort previous request if loading
    if (generating && abortController) {
      abortController.abort();
      setAbortController(null);
      setGenerating(false);
      return;
    }

    setGenerating(true);
    setError(null);

    const controller = new AbortController();
    setAbortController(controller);

    try {
      const data = await generateContext({
        id: caseId,
        ground_truth: gtText
      }, controller.signal);

      setContext(data);
      // generateHashes(data); // This function is not defined in the original code. Removing it.
      setGtAtGeneration(gtText);
    } catch (err: any) {
      if (err.name !== 'AbortError') {
        setError(err.message);
      }
    } finally {
      if (!controller.signal.aborted) {
        setGenerating(false);
        setAbortController(null);
      }
    }
  };

  const hasChanges = () => {
    if (!initialContext || !context) return false;
    const gtChanged = gtText !== initialGT;
    // Simple check for context changes - deeper check could be expensive but check ref or content
    // Checkpoints length or content
    const contextChanged = JSON.stringify(context.checkpoints) !== JSON.stringify(initialContext.checkpoints)
      || context.meta.business_goal !== initialContext.meta.business_goal
      || context.meta.audio_reality_inference !== initialContext.meta.audio_reality_inference;

    return gtChanged || contextChanged;
  };

  const handlePrimaryAction = () => {
    if (initialContext) {
      if (hasChanges()) {
        setView('COMPARISON');
      }
    } else {
      handleSaveDirectly();
    }
  };

  const handleSaveDirectly = async () => {
    if (!context) return;
    // optimistic update in parent?
    onSave(context, gtText);
    try {
      // UpdateContext
      await updateContext({
        id: caseId,
        eval_context: { ...context, meta: { ...context.meta, ground_truth: gtText } }
      });
      onClose();
    } catch (err: any) {
      setError("Failed to save: " + err.message);
    }
  };

  const getTitle = () => {
    if (view === 'COMPARISON') return 'Review Changes';
    return initialContext ? 'Edit Evaluation Context' : 'Create Evaluation Context';
  };

  const handleCheckpointClick = (checkpoint: Checkpoint) => {
    if (audioPlayerRef.current && checkpoint.start_ms !== undefined) {
      audioPlayerRef.current.seek(checkpoint.start_ms / 1000);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm">
      <div className="bg-white border border-slate-200 rounded-xl shadow-2xl w-[90vw] h-[80vh] flex flex-col overflow-hidden">
        {/* Header */}
        <div className="flex items-center gap-4 px-6 py-3 border-b border-slate-200 bg-white shrink-0">
          <span className="text-sm font-bold text-slate-700 shrink-0">
            {getTitle()}
          </span>
          <AudioPlayer ref={audioPlayerRef} caseId={caseId} className="flex-1" />
          <button onClick={onClose} className="p-1.5 hover:bg-slate-100 rounded text-slate-400 hover:text-slate-600 transition-colors shrink-0">
            <X size={18} />
          </button>
        </div>

        {/* Content */}
        {view === 'EDITOR' ? (
          <ContextCreator
            gtText={gtText}
            setGtText={setGtText}
            gtAtGeneration={gtAtGeneration}
            context={context}
            loading={generating}
            error={error}
            onGenerate={handleGenerate}
            onPrimaryAction={handlePrimaryAction}
            initialContext={initialContext}
            onCancel={onClose}
            disablePrimary={initialContext ? !hasChanges() : false}
            onCheckpointClick={handleCheckpointClick}
          />
        ) : (
          <ContextReviewer
            oldContext={initialContext!}
            newContext={context!}
            oldGT={initialGT}
            newGT={gtText}
            onBack={() => setView('EDITOR')}
            onSave={handleSaveDirectly}
            onCancel={onClose}
            onCheckpointClick={handleCheckpointClick}
          />
        )}
      </div>
    </div>
  );
};
