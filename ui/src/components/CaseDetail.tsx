import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import { AlertTriangle, Settings, Copy, Play } from 'lucide-react';
import { Case, LoadingData, EvalContext } from '../types';
import { EvalReportView } from './EvalReportView';
import { ContextManagerModal } from './ContextManagerModal';
import { RichTooltip } from './RichTooltip';
import { EvalContextDisplay } from './EvalContextDisplay';
import { AudioPlayer, AudioPlayerHandle } from './AudioPlayer';
import { Checkpoint } from '../types';

interface CaseDetailProps {
  onEvalComplete: () => void;
  processingCases: Set<string>;
  startProcessing: (id: string) => void;
  endProcessing: (id: string) => void;
  getSelection: (id: string) => Record<string, boolean>;
  setSelectionForCase: (id: string, newVal: Record<string, boolean>) => void;
  initSelection: (data: LoadingData) => Record<string, boolean>;
}

export function CaseDetail({ onEvalComplete, processingCases, startProcessing, endProcessing, getSelection, setSelectionForCase, initSelection }: CaseDetailProps) {
  const { id } = useParams<{ id: string }>();
  const idRef = useRef(id);
  const [currentCase, setCurrentCase] = useState<Case | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedProviders, setSelectedProviders] = useState<Record<string, boolean>>({});
  const [isContextModalOpen, setIsContextModalOpen] = useState(false);
  const audioPlayerRef = useRef<AudioPlayerHandle>(null);

  const mounted = useRef(true);
  useEffect(() => {
    idRef.current = id;
    mounted.current = true;
    return () => { mounted.current = false; };
  }, [id]);

  const loadCaseData = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      const res = await fetch(`/api/case?id=${id}`);
      if (!res.ok) throw new Error("Failed to load case");
      const data = await res.json();
      if (mounted.current && idRef.current === id) {
        setCurrentCase(data);
        if (id && processingCases.has(id)) {
          const persisted = getSelection(id);
          setSelectedProviders(persisted || initSelection(data));
        } else {
          setSelectedProviders(initSelection(data));
        }
      }
    } catch (e) {
      console.error(e);
    } finally {
      if (mounted.current && idRef.current === id && !silent) setLoading(false);
    }
  }, [id, processingCases, getSelection, initSelection]);

  useEffect(() => {
    if (id) loadCaseData();
  }, [id, loadCaseData]);



  const updateCaseLocal = (updates: Partial<Case>) => {
    if (mounted.current) {
      setCurrentCase(prev => prev ? ({ ...prev, ...updates }) : null);
    }
  };

  const toggleProviderSelection = (provider: string) => {
    setSelectedProviders(prev => ({
      ...prev,
      [provider]: !prev[provider]
    }));
  };

  const runEval = async () => {
    if (!currentCase) return;
    const gt = currentCase.ground_truth || "";
    if (!gt.trim()) return alert("Ground Truth required");

    const evalId = currentCase.id;
    const providersToEval = Object.keys(selectedProviders).filter(s => selectedProviders[s]);
    if (providersToEval.length === 0) return alert("Select at least one provider");

    const resultsToEval: Record<string, string> = {};
    Object.keys(currentCase.transcripts).forEach(provider => {
      if (selectedProviders[provider]) {
        resultsToEval[provider] = currentCase.transcripts[provider];
      }
    });

    if (Object.keys(resultsToEval).length === 0) {
      return alert("Please select at least one provider to evaluate.");
    }

    setSelectionForCase(evalId, selectedProviders);
    startProcessing(evalId);

    try {
      const res = await fetch('/api/evaluate-v2', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: evalId,
          eval_context: currentCase.eval_context,
          transcripts: resultsToEval
        })
      });
      if (!res.ok) throw await res.text();
      if (idRef.current === evalId) await loadCaseData(true);
      if (onEvalComplete) onEvalComplete();
    } catch (e) {
      alert("Eval Failed: " + e);
    } finally {
      endProcessing(evalId);
    }
  };

  const handleContextSave = (ctx: EvalContext, gt: string) => {
    updateCaseLocal({
      eval_context: ctx,
      ground_truth: gt,
      // Clear reports to reflect that they are now stale/invalid until re-run
      report_v2: undefined
    });
  };

  const isProcessingThisCase = currentCase?.id ? processingCases.has(currentCase.id) : false;
  const evalContext = currentCase?.eval_context;

  const handleCheckpointClick = (checkpoint: Checkpoint) => {
    if (audioPlayerRef.current && checkpoint.start_ms !== undefined) {
      audioPlayerRef.current.seek(checkpoint.start_ms / 1000);
    }
  };

  if (loading) return <div className="p-8 text-center text-slate-500">Loading case data...</div>;
  if (!currentCase) return <div className="p-8 text-center text-slate-500">Case not found.</div>;

  return (
    <>
      {/* Header: Case ID + Audio Player */}
      <div className="bg-white border-b border-slate-200 px-8 py-3 shrink-0 flex items-center gap-4">
        <div className="flex items-center gap-2">
          <span className="text-sm font-mono font-medium text-slate-700 select-all">{currentCase.id}</span>
          <button
            onClick={() => navigator.clipboard.writeText(currentCase.id)}
            className="p-1 hover:bg-slate-100 rounded text-slate-400 hover:text-primary transition-colors"
            title="Copy Case ID"
          >
            <Copy size={12} />
          </button>
        </div>

        <AudioPlayer ref={audioPlayerRef} caseId={currentCase.id} className="flex-1 max-w-md" />

        {/* Action Buttons */}
        <div className="flex items-center gap-3 ml-auto">
          {/* Compact Questionable GT Warning */}
          {evalContext?.meta.questionable_gt && (
            <RichTooltip
              trigger={
                <div className="flex items-center gap-1.5 px-3 py-1.5 bg-amber-50 border border-amber-200 rounded-lg shadow-sm animate-pulse cursor-help">
                  <AlertTriangle size={12} className="text-amber-600" />
                  <span className="text-xs font-bold text-amber-800 uppercase tracking-tight">GT Quality Alert</span>
                </div>
              }
            >
              <div className="p-4 max-w-xs">
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
            <Settings size={12} /> {evalContext ? 'Manage Context' : 'Create Context'}
          </button>
          <button
            onClick={runEval}
            disabled={isProcessingThisCase || Object.values(selectedProviders).filter(Boolean).length === 0}
            className="bg-primary hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed text-white px-3 py-1.5 rounded-lg text-xs font-bold flex items-center gap-2 transition-colors shadow-sm"
          >
            {isProcessingThisCase ? 'Evaluating...' : <><Play size={12} /> Evaluate ({Object.values(selectedProviders).filter(Boolean).length})</>}
          </button>
        </div>
      </div>

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
      <div className="flex-1 flex flex-col min-h-0 bg-white relative">
        <EvalReportView
          key={currentCase.id}
          kase={currentCase}
          selectedProviders={selectedProviders}
          onToggleProvider={toggleProviderSelection}
          onSelectAll={() => {
            const allProviders = new Set([
              ...Object.keys(currentCase.transcripts || {}),
            ]);
            const newSelection: Record<string, boolean> = {};
            allProviders.forEach(p => newSelection[p] = true);
            setSelectedProviders(newSelection);
          }}
          onDeselectAll={() => setSelectedProviders({})}
          onSelectDefault={() => setSelectedProviders(initSelection(currentCase))}
          getDefaultSelection={() => initSelection(currentCase)}
          isProcessing={isProcessingThisCase}
        />
      </div>

      {/* Context Manager Modal */}
      <ContextManagerModal
        isOpen={isContextModalOpen}
        onClose={() => setIsContextModalOpen(false)}
        caseId={currentCase.id}
        initialGT={currentCase.eval_context?.meta.ground_truth || currentCase.ground_truth || ""}
        initialContext={currentCase.eval_context}
        onSave={handleContextSave}
      />
    </>
  );
}
