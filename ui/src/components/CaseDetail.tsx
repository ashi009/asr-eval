import { useState, useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { Copy, AlertTriangle, Settings, Play, Loader2 } from 'lucide-react';
import { useWorkspace, useCase } from '../workspace/context';
import { AudioPlayer } from './AudioPlayer';
import { RichTooltip } from './RichTooltip';
import { EvalContextDisplay } from './EvalContextDisplay';
import { EvalReportView } from './EvalReportView';
import { ContextManagerModal } from './ContextManagerModal';
import { Case, Checkpoint } from '../workspace/types';

interface CaseDetailProps {
  onEvalComplete?: () => void;
  processingCases: Set<string>;
  startProcessing: (id: string) => void;
  endProcessing: (id: string) => void;
  getSelection: (id: string) => Record<string, boolean>;
  setSelectionForCase: (id: string, val: Record<string, boolean>) => void;
  initSelection: (data: Case) => Record<string, boolean>;
}

export function CaseDetail({
  onEvalComplete,
  processingCases,
  startProcessing,
  endProcessing,
  getSelection,
  setSelectionForCase,
  initSelection
}: CaseDetailProps) {
  const { id } = useParams<{ id: string }>();
  const { currentCase, loading, error, refresh } = useCase(id);
  const { evaluateCase } = useWorkspace();
  const [isContextModalOpen, setIsContextModalOpen] = useState(false);
  const audioPlayerRef = useRef<{ seek: (t: number) => void; pause: () => void }>(null);
  const idRef = useRef(id);

  // Sync idRef and ensure selection init
  useEffect(() => {
    idRef.current = id;
  }, [id]);

  useEffect(() => {
    if (currentCase && id && !getSelection(id)) {
      setSelectionForCase(id, initSelection(currentCase));
    }
  }, [currentCase, id, getSelection, setSelectionForCase, initSelection]);

  const toggleProvider = (provider: string) => {
    if (!id) return;
    const current = getSelection(id) || {};
    setSelectionForCase(id, { ...current, [provider]: !current[provider] });
  };

  const selectedProviders = (id ? getSelection(id) : {}) || {};

  const runEval = async () => {
    if (!currentCase || !id) return;
    const gt = currentCase.eval_context?.meta?.ground_truth || "";
    if (!gt.trim()) return alert("Ground Truth required (create context first)");

    const providersToEval = Object.keys(selectedProviders).filter(s => selectedProviders[s]);
    if (providersToEval.length === 0) return alert("Select at least one provider");

    // We only need provider IDs now, as transcripts are loaded/managed by backend
    // or passed via map if we want to override (but we rely on backend loading).
    // Actually, backend loads transcripts. Use provider_ids.

    startProcessing(id);
    try {
      await evaluateCase({
        id: id,
        eval_context: currentCase.eval_context!,
        provider_ids: providersToEval
      });
      if (idRef.current === id) await refresh();
      if (onEvalComplete) onEvalComplete();
    } catch (e: any) {
      alert("Eval Failed: " + e.message);
    } finally {
      endProcessing(id);
    }
  };

  const handleContextSave = () => {
    refresh();
  };

  const handleCheckpointClick = (checkpoint: Checkpoint) => {
    if (audioPlayerRef.current && checkpoint.start_ms !== undefined) {
      audioPlayerRef.current.seek(checkpoint.start_ms / 1000);
    }
  };

  if (loading) return (
    <div className="flex-1 flex items-center justify-center text-slate-400">
      <Loader2 className="w-8 h-8 animate-spin mb-2" />
      <p>Loading case...</p>
    </div>
  );

  if (error) return (
    <div className="flex-1 flex items-center justify-center text-red-500">
      <p>Error: {error}</p>
    </div>
  );

  if (!currentCase) return (
    <div className="flex-1 flex items-center justify-center text-slate-400">
      <p>Case not found</p>
    </div>
  );

  const evalContext = currentCase.eval_context;
  const isProcessingThisCase = processingCases.has(currentCase.id);

  return (
    <div className="flex flex-col h-full bg-slate-50">
      {/* Header: Case ID + Audio Player */}
      <div className="bg-white border-b border-slate-200 px-6 py-3 shrink-0 flex items-center gap-4 shadow-sm z-10">
        <div className="flex items-center gap-2">
          <span className="text-sm font-mono font-medium text-slate-700 select-all">{currentCase.id}</span>
          <button
            onClick={() => navigator.clipboard.writeText(currentCase.id)}
            className="p-1 hover:bg-slate-100 rounded text-slate-400 hover:text-primary transition-colors"
            title="Copy Case ID"
          >
            <Copy size={14} />
          </button>
        </div>

        <AudioPlayer ref={audioPlayerRef} caseId={currentCase.id} className="flex-1 max-w-xl mx-auto" />

        {/* Action Buttons */}
        <div className="flex items-center gap-3 ml-auto">
          {/* Compact Questionable GT Warning */}
          {evalContext?.meta?.questionable_gt && (
            <RichTooltip
              trigger={
                <div className="flex items-center gap-1.5 px-3 py-1.5 bg-amber-50 border border-amber-200 rounded-lg shadow-sm cursor-help">
                  <AlertTriangle size={14} className="text-amber-600" />
                  <span className="text-xs font-bold text-amber-800 uppercase tracking-tight">GT Quality Alert</span>
                </div>
              }
            >
              <div className="p-3 max-w-xs">
                <p className="text-xs text-slate-600 leading-relaxed font-medium">
                  {evalContext.meta.questionable_reason || 'The ground truth may have issues.'}
                </p>
              </div>
            </RichTooltip>
          )}

          <button
            onClick={() => {
              audioPlayerRef.current?.pause();
              setIsContextModalOpen(true);
            }}
            className="px-3 py-1.5 bg-white border border-slate-200 hover:border-slate-300 hover:bg-slate-50 text-slate-700 text-xs font-medium rounded-lg shadow-sm transition-all flex items-center gap-2"
          >
            <Settings size={14} /> {evalContext ? 'Manage Context' : 'Create Context'}
          </button>

          <button
            onClick={runEval}
            disabled={isProcessingThisCase || Object.values(selectedProviders).filter(Boolean).length === 0}
            className="bg-primary hover:bg-primary-dark disabled:opacity-50 disabled:cursor-not-allowed text-white px-3 py-1.5 rounded-lg text-xs font-bold flex items-center gap-2 transition-colors shadow-sm"
          >
            {isProcessingThisCase ? (
              <><Loader2 size={14} className="animate-spin" /> Evaluating...</>
            ) : (
              <><Play size={14} /> Evaluate ({Object.values(selectedProviders).filter(Boolean).length})</>
            )}
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        <div className="min-h-full flex flex-col">
          {/* Context View Panel - Business Goal + GT with Checkpoints */}
          {evalContext && (
            <EvalContextDisplay
              context={evalContext}
              enableAudioRealityToggle={true}
              showWeightInBadge={false}
              onCheckpointClick={handleCheckpointClick}
              className="bg-slate-50 border-b border-slate-200 px-8 py-4 shrink-0"
            />
          )}

          {/* No Context - Show CTA inline */}
          {!evalContext && (
            <div className="bg-amber-50 border-b border-amber-200 px-8 py-6 shrink-0 text-center">
              <AlertTriangle size={24} className="text-amber-500 mx-auto mb-2" />
              <h3 className="text-sm font-bold text-amber-800 mb-1">No Evaluation Context</h3>
              <p className="text-xs text-amber-600 mb-3">Generate context from ground truth to see checkpoints and enable evaluation.</p>
              <button
                onClick={() => {
                  audioPlayerRef.current?.pause();
                  setIsContextModalOpen(true);
                }}
                className="px-4 py-2 bg-amber-500 hover:bg-amber-600 text-white text-sm font-bold rounded-lg shadow-sm transition-all"
              >
                Create Context
              </button>
            </div>
          )}

          {/* Eval Report View */}
          <div className="flex-1 bg-white relative">
            {(currentCase.report_v2 || (currentCase.transcripts && Object.keys(currentCase.transcripts).length > 0)) ? (
              <EvalReportView
                key={currentCase.id}
                kase={currentCase} // Pass kase as simplified ReportView expects
                selectedProviders={selectedProviders}
                onToggleProvider={toggleProvider}
                onSelectAll={() => {
                  const newSelection: Record<string, boolean> = {};
                  Object.keys(currentCase.transcripts || {}).forEach(p => newSelection[p] = true);
                  setSelectionForCase(id!, newSelection);
                }}
                onDeselectAll={() => setSelectionForCase(id!, {})}
                onSelectDefault={() => setSelectionForCase(id!, initSelection(currentCase))}
                getDefaultSelection={() => initSelection(currentCase)}
                isProcessing={isProcessingThisCase}
              />
            ) : (
              <div className="text-center py-20 text-slate-400">
                <p>No transcripts or evaluation report available.</p>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Context Manager Modal */}
      {isContextModalOpen && (
        <ContextManagerModal
          isOpen={isContextModalOpen}
          onClose={() => setIsContextModalOpen(false)}
          caseId={currentCase.id}
          initialGT={currentCase.eval_context?.meta?.ground_truth || ""}
          initialContext={currentCase.eval_context}
          onSave={handleContextSave}
        />
      )}
    </div>
  );
}
