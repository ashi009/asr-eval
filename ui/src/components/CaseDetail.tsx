import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import { Play, Pause, Check, AlertTriangle, ArrowRight, X } from 'lucide-react';
import { Case, LoadingData } from '../types';
import { formatTime } from '../utils/formatUtils';
import { renderDiff } from './DiffRenderer';
import { EvalReportView } from './EvalReportView';

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

  const [isInputExpanded, setIsInputExpanded] = useState(true);

  const mounted = useRef(true);
  useEffect(() => {
    idRef.current = id;
    mounted.current = true;
    return () => { mounted.current = false; };
  }, [id]);

  const audioRef = useRef<HTMLAudioElement>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [showResolveModal, setShowResolveModal] = useState(false);

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
          if (persisted) {
            setSelectedProviders(persisted);
          } else {
            setSelectedProviders(initSelection(data));
          }
        } else {
          setSelectedProviders(initSelection(data));
        }

        if (data.ground_truth?.trim()) {
          setIsInputExpanded(false);
        } else {
          setIsInputExpanded(true);
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

  useEffect(() => {
    // Reset player state when case changes
    setIsPlaying(false);
    setCurrentTime(0);
    setDuration(0);
  }, [currentCase?.id]);

  const togglePlay = () => {
    if (!audioRef.current) return;
    if (audioRef.current.paused) {
      audioRef.current.play();
      setIsPlaying(true);
    } else {
      audioRef.current.pause();
      setIsPlaying(false);
    }
  };

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
      const res = await fetch('/api/evaluate-llm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: evalId,
          ground_truth: gt,
          transcripts: resultsToEval
        })
      });
      if (!res.ok) throw await res.text();

      if (idRef.current === evalId) {
        await loadCaseData(true); // reload silently (keep loading spinner managed by processingCases if any? Or silent is fine)
      }
      if (onEvalComplete) onEvalComplete();
    } catch (e) {
      alert("Eval Failed: " + e);
    } finally {
      endProcessing(evalId);
    }
  };

  const isProcessingThisCase = currentCase?.id ? processingCases.has(currentCase.id) : false;

  const currentGT = currentCase?.ground_truth;
  const evalReport = currentCase?.eval_report;
  const evalGT = evalReport?.ground_truth;

  if (loading) return <div className="p-8 text-center text-slate-500">Loading case data...</div>;
  if (!currentCase) {
    return <div className="p-8 text-center text-slate-500">Case not found.</div>;
  }

  return (
    <>
      <div className="flex-1 flex flex-col min-h-0 bg-white relative">
        <EvalReportView
          key={currentCase.id}
          kase={currentCase}
          selectedProviders={selectedProviders}
          onToggleProvider={toggleProviderSelection}
          onSelectAll={() => {
            const allProviders = new Set([
              ...Object.keys(currentCase.transcripts || {}),
              ...Object.keys(currentCase.eval_report?.eval_results || {})
            ]);
            const newSelection: Record<string, boolean> = {};
            allProviders.forEach(p => newSelection[p] = true);
            setSelectedProviders(newSelection);
          }}
          onDeselectAll={() => {
            setSelectedProviders({});
          }}
          onSelectDefault={() => {
            setSelectedProviders(initSelection(currentCase));
          }}
          getDefaultSelection={() => initSelection(currentCase)}
        />
      </div>

      <div className="bg-white border-t border-slate-200 px-8 py-4 shrink-0 z-10 transition-all duration-300">
        <div className="flex items-center gap-4 mb-4 border-b border-slate-100 pb-3">
          <button onClick={togglePlay} className="w-8 h-8 rounded-full bg-slate-900 text-white flex items-center justify-center hover:opacity-90 shrink-0">
            {isPlaying ? <Pause size={16} fill="currentColor" /> : <Play size={16} fill="currentColor" />}
          </button>
          <div className="flex-1">
            <div className="flex justify-between text-[10px] font-medium text-slate-500 mb-1">
              <span className="font-bold text-slate-700">Playback</span>
              <span className="font-mono">{formatTime(currentTime)} / {formatTime(duration)}</span>
            </div>
            <div
              className="relative h-1.5 bg-slate-100 rounded-full cursor-pointer group"
              onClick={e => {
                const rect = e.currentTarget.getBoundingClientRect();
                const pct = (e.clientX - rect.left) / rect.width;
                if (audioRef.current) {
                  audioRef.current.currentTime = pct * duration;
                  if (audioRef.current.paused) {
                    audioRef.current.play();
                    setIsPlaying(true);
                  }
                }
              }}
            >
              <div className="absolute top-0 left-0 h-full bg-primary rounded-full transition-all" style={{ width: `${(currentTime / duration) * 100}%` }} />
            </div>
          </div>
          <audio
            key={currentCase.id}
            ref={audioRef}
            src={`/audio/${currentCase.id}.flac`}
            onTimeUpdate={e => setCurrentTime(e.currentTarget.currentTime)}
            onLoadedMetadata={e => setDuration(e.currentTarget.duration)}
            onEnded={() => setIsPlaying(false)}
          />
        </div>

        <div className="flex flex-col gap-2">
          <div className="flex justify-between items-center">
            <div
              className="flex items-center gap-2 cursor-pointer select-none"
              onClick={() => setIsInputExpanded(!isInputExpanded)}
            >
              <label className="text-sm font-bold flex items-center gap-2 cursor-pointer">
                <Check className={`w-4 h-4 transition-colors ${currentGT ? 'text-green-500' : 'text-slate-400'}`} />
                Ground Truth
              </label>
              <span className="text-xs text-slate-400 hover:text-primary transition-colors">
                {isInputExpanded ? '(Click to collapse)' : '(Click to expand)'}
              </span>
            </div>
            <div className="flex items-center gap-2 ml-auto">
              {evalGT && currentGT !== evalGT ? (
                <button
                  onClick={() => setShowResolveModal(true)}
                  className="bg-amber-500 hover:bg-amber-600 text-white px-3 py-1 rounded text-xs font-bold flex items-center gap-2 transition-colors shadow-sm animate-in fade-in zoom-in-95 duration-200"
                >
                  <AlertTriangle size={12} /> Review
                </button>
              ) : (
                <button
                  onClick={runEval}
                  disabled={isProcessingThisCase || Object.values(selectedProviders).filter(Boolean).length === 0}
                  className="bg-primary hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed text-white px-3 py-1 rounded text-xs font-bold flex items-center gap-2 transition-colors shadow-sm"
                >
                  {isProcessingThisCase ? 'Running...' : <><Play size={12} /> Run AI Eval ({Object.values(selectedProviders).filter(Boolean).length})</>}
                </button>
              )}
            </div>
          </div>

          {isInputExpanded ? (
            <textarea
              className="w-full h-32 border border-slate-200 rounded p-3 text-sm font-mono focus:ring-1 focus:ring-primary focus:border-primary disabled:bg-slate-50 disabled:text-slate-500 animate-in fade-in zoom-in-95 duration-200"
              placeholder="Enter ground truth..."
              value={currentGT || ""}
              onChange={async e => {
                const newVal = e.target.value;
                updateCaseLocal({ ground_truth: newVal });
              }}
              disabled={isProcessingThisCase}
              autoFocus
            />
          ) : (
            <div
              className="w-full bg-slate-50 border border-slate-200 rounded p-2 text-sm text-slate-600 font-mono italic cursor-pointer hover:border-slate-300 hover:bg-slate-100 transition-colors truncate"
              onClick={() => setIsInputExpanded(true)}
              title={currentCase.ground_truth}
            >
              {currentCase.ground_truth || "No ground truth provided"}
            </div>
          )}
        </div>
      </div >

      {evalGT && currentGT !== evalGT && showResolveModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm animate-in fade-in duration-200">
          <div ref={modalRef => {
            const handleClickOutside = (e: MouseEvent) => {
              if (modalRef && !modalRef.contains(e.target as Node)) {
                setShowResolveModal(false);
              }
            };
            document.addEventListener('mousedown', handleClickOutside);
            return () => document.removeEventListener('mousedown', handleClickOutside);
          }} className="bg-white rounded-lg shadow-2xl w-full max-w-2xl border border-slate-200 overflow-hidden animate-in zoom-in-95 duration-200">
            <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between bg-slate-50/50">
              <div className="flex items-center gap-2 font-bold text-amber-600">
                <AlertTriangle size={20} />
                <span>Review Ground Truth Diff</span>
              </div>
              <button onClick={() => setShowResolveModal(false)} className="text-slate-400 hover:text-slate-600">
                <X size={20} />
              </button>
            </div>

            <div className="p-6 space-y-6">
              <p className="text-sm text-slate-600">
                The Ground Truth has been modified since the last evaluation. You must resolve this difference before running a new evaluation.
              </p>

              <div className="space-y-2">
                <div className="flex items-center gap-2 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                  <span>Evaluated</span>
                  <ArrowRight size={10} />
                  <span>Current</span>
                </div>
                <div className="text-sm font-mono bg-slate-50 p-4 rounded border border-slate-200 max-h-64 overflow-auto whitespace-pre-wrap break-all leading-relaxed">
                  {renderDiff(evalGT, currentGT)}
                </div>
              </div>

              <div className="flex gap-3 justify-end pt-2">
                <button
                  className="px-4 py-2 bg-white border border-slate-300 rounded text-sm font-semibold hover:bg-slate-50 shadow-sm transition-colors text-slate-700"
                  onClick={async () => {
                    const oldGt = evalGT;
                    if (!oldGt) return;
                    setLoading(true);
                    try {
                      const res = await fetch('/api/save-gt', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ id: currentCase.id, ground_truth: oldGt })
                      });
                      if (!res.ok) throw await res.text();
                      updateCaseLocal({ ground_truth: oldGt });
                      setShowResolveModal(false);
                    } catch (e) {
                      console.error(e);
                      alert("Failed to revert: " + e);
                    } finally {
                      setLoading(false);
                    }
                  }}
                >
                  Revert
                </button>
                <button
                  className="px-4 py-2 bg-amber-600 border border-amber-700 rounded text-sm font-semibold text-white hover:bg-amber-700 shadow-sm transition-colors"
                  onClick={async () => {
                    setLoading(true);
                    try {
                      const res = await fetch(`/api/reset-eval?id=${currentCase.id}&t=${Date.now()}`, { method: 'POST' });
                      if (!res.ok) throw new Error("Failed");
                      const fresh = await fetch(`/api/case?id=${currentCase.id}&t=${Date.now()}`).then(r => r.json());
                      setCurrentCase({ ...fresh, ground_truth: currentGT });
                      setSelectedProviders(initSelection(fresh));
                      if (currentGT?.trim()) setIsInputExpanded(false);
                      setShowResolveModal(false);
                    } catch (e) { console.error(e); } finally { setLoading(false); }
                  }}
                >
                  Reset Eval Report
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
