import { useState, useEffect, useRef, useCallback } from 'react';
import { BrowserRouter, Routes, Route, NavLink, useParams } from 'react-router-dom';
import { Play, Pause, Search, Check, AlertTriangle, Minus, X, AudioLines, Loader2, Copy, ArrowRight } from 'lucide-react';
import { getASRProviderConfig } from './config';
import { smartDiff } from './diffUtils';

/* --- Interfaces --- */

interface EvalResult {
  score: number;
  revised_transcript?: string;
  transcript?: string;
  summary?: string[];
}

interface EvalReport {
  ground_truth: string;
  eval_results: Record<string, EvalResult>;
}

interface Case {
  id: string;
  eval_report?: EvalReport;
  ground_truth?: string;
  transcripts: Record<string, string>;
  has_ai?: boolean;
  best_performers?: string[];
}



interface LoadingData {
  id: string;
  eval_report?: EvalReport;
  results?: Record<string, EvalResult>;
  transcripts: Record<string, string>;
  isLoading?: boolean;
  error?: string;
  has_ai?: boolean;
  ground_truth?: string;
  evaluated_ground_truth?: string;
}

/* --- Helper: Diff Render --- */
const renderDiff = (original: string, revised?: string) => {
  if (revised === undefined || revised === null) return original;

  const diffs = smartDiff(original, revised, true);

  return (
    <span>
      {diffs.map((part, index) => {
        if (part.added) {
          return (
            <span key={index} className="bg-green-100 text-green-700 font-medium px-0.5 rounded mx-0.5 animate-in fade-in duration-300 select-none">
              {part.value}
            </span>
          );
        } else if (part.removed) {
          return (
            <span key={index} className="bg-red-50 text-red-400 line-through decoration-red-400/50 px-0.5 rounded mx-0.5 opacity-60">
              {part.value}
            </span>
          );
        }
        return <span key={index}>{part.value}</span>;
      })}
    </span>
  );
};

const isResultStale = (currentTranscript: string | undefined, result: EvalResult | undefined) => {
  if (!result?.transcript) return false;
  return currentTranscript !== result.transcript;
};

function Layout() {
  const [cases, setCases] = useState<Case[]>([]);
  const [search, setSearch] = useState("");
  const [processingCases, setProcessingCases] = useState<Set<string>>(new Set());
  const [caseSelections, setCaseSelections] = useState<Record<string, Record<string, boolean>>>({});

  const initSelection = (data: LoadingData) => {
    const initialSelection: Record<string, boolean> = {};

    const results = data.eval_report?.eval_results || data.results || {};

    const providerKeys = new Set([
      ...Object.keys(data.transcripts || {}),
      ...Object.keys(results)
    ]);

    providerKeys.forEach(provider => {
      const config = getASRProviderConfig(provider);
      const isEnabled = config.enabled !== false;

      const hasResult = !!results[provider];
      const isStale = hasResult && isResultStale(data.transcripts?.[provider], results[provider]);

      initialSelection[provider] = isEnabled && (!hasResult || isStale);
    });
    return initialSelection;
  };

  const getSelection = (id: string) => caseSelections[id];

  const setSelectionForCase = (id: string, newVal: Record<string, boolean>) => {
    setCaseSelections(prev => ({ ...prev, [id]: newVal }));
  };

  const startProcessing = (id: string) => {
    setProcessingCases(prev => new Set(prev).add(id));
  };

  const endProcessing = (id: string) => {
    setProcessingCases(prev => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  };

  const loadCases = async () => {
    try {
      const res = await fetch('/api/cases');
      const data = await res.json();
      data.sort((a: Case, b: Case) => a.id.localeCompare(b.id));
      setCases(data);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    loadCases();
  }, []);

  const filteredCases = cases.filter(c => c.id.toLowerCase().includes(search.toLowerCase()));
  const pendingCases = filteredCases.filter(c => !c.has_ai);
  const doneCases = filteredCases.filter(c => c.has_ai);

  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [isOverlay, setIsOverlay] = useState(false);
  const [llmModel, setLlmModel] = useState<string>("");

  useEffect(() => {
    fetch('/api/config')
      .then(res => res.json())
      .then(data => setLlmModel(data.llm_model))
      .catch(console.error);
  }, []);

  useEffect(() => {
    const handleResize = () => {
      const overlay = window.innerWidth < 1280;
      setIsOverlay(overlay);
      if (overlay) {
        setSidebarOpen(false);
      } else {
        setSidebarOpen(true);
      }
    };
    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const toggleSidebar = () => setSidebarOpen(!sidebarOpen);
  const handleLinkClick = () => {
    if (isOverlay) setSidebarOpen(false);
  };

  return (
    <div className="flex h-screen bg-background-light dark:bg-background-dark font-display text-slate-900 overflow-hidden relative">
      {/* Backdrop for Overlay Mode */}
      {isOverlay && sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/20 backdrop-blur-sm z-[49] transition-opacity"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <aside
        className={`border-r border-slate-200 bg-slate-50 flex flex-col shrink-0 transition-all duration-300 ease-in-out z-[50]
          ${isOverlay
            ? `fixed inset-y-0 left-0 h-full shadow-2xl ${sidebarOpen ? 'translate-x-0 w-80' : '-translate-x-full w-80'}`
            : `relative ${sidebarOpen ? 'w-80 translate-x-0' : 'w-0 -translate-x-full opacity-0 overflow-hidden'}`
          }
        `}
      >
        <div className="p-4 border-b border-slate-200 bg-white w-full">
          <div className="flex items-center gap-2 mb-4 justify-between">
            <div className="flex flex-col gap-0.5">
              <div className="flex items-center gap-2">
                <AudioLines className="text-primary" />
                <h1 className="font-bold">ASR Eval Pro</h1>
              </div>
              {llmModel && (
                <div className="text-[10px] font-mono text-slate-500 pl-8 opacity-80">
                  {llmModel}
                </div>
              )}
            </div>
            <button onClick={toggleSidebar} className="p-1 hover:bg-slate-100 rounded lg:hidden">
              <span className="sr-only">Close sidebar</span>
              <div className="w-4 h-4 flex flex-col justify-between">
                <span className="w-full h-0.5 bg-slate-400 block origin-center transform rotate-45 translate-y-[6px]"></span>
                <span className="w-full h-0.5 bg-slate-400 block opacity-0"></span>
                <span className="w-full h-0.5 bg-slate-400 block origin-center transform -rotate-45 -translate-y-[6px]"></span>
              </div>
            </button>
          </div>
          <div className="relative">
            <Search className="absolute left-2 top-2 text-slate-400 w-4 h-4" />
            <input
              className="w-full bg-slate-100 border-none rounded pl-8 py-1.5 text-sm"
              placeholder="Search..."
              value={search}
              onChange={e => setSearch(e.target.value)}
            />
          </div>
        </div>
        <div className="flex-1 overflow-y-auto p-3 space-y-6 w-full">
          {pendingCases.length > 0 && (
            <div>
              <h3 className="text-xs font-bold text-slate-500 uppercase px-2 mb-2">Pending</h3>
              <ul className="space-y-1">
                {pendingCases.map(c => (
                  <li key={c.id} className="min-w-0">
                    <NavLink
                      to={`/case/${c.id}`}
                      onClick={handleLinkClick}
                      className={({ isActive }) => `block w-full text-left px-3 py-2 rounded text-sm font-mono flex justify-between
                        ${isActive ? 'bg-white shadow ring-1 ring-slate-200' : 'hover:bg-slate-200/50 text-slate-600'}
                      `}
                    >
                      <div className="flex items-center gap-2 min-w-0 flex-1">
                        <span className="truncate block">{c.id}</span>
                        {processingCases.has(c.id) && <Loader2 className="w-3 h-3 animate-spin text-primary shrink-0" />}
                      </div>
                    </NavLink>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {doneCases.length > 0 && (
            <div>
              <h3 className="text-xs font-bold text-slate-500 uppercase px-2 mb-2">Done</h3>
              <ul className="space-y-1">
                {doneCases.map(c => {
                  return (
                    <li key={c.id}>
                      <NavLink
                        to={`/case/${c.id}`}
                        onClick={handleLinkClick}
                        className={({ isActive }) => `block w-full text-left px-3 py-2 rounded text-sm font-mono flex justify-between items-center group
                          ${isActive ? 'bg-white shadow ring-1 ring-slate-200 border-l-2 border-primary' : 'hover:bg-slate-200/50 text-slate-600 border-l-2 border-transparent'}
                        `}
                      >
                        <div className="flex items-center gap-2 min-w-0 flex-1">
                          <span className="truncate block">{c.id}</span>
                          {processingCases.has(c.id) && <Loader2 className="w-3 h-3 animate-spin text-primary shrink-0" />}
                        </div>
                        <div className="flex items-center -space-x-1 shrink-0 ml-2">
                          {c.best_performers?.length ? (
                            c.best_performers.map((p, idx) => {
                              const config = getASRProviderConfig(p);
                              return (
                                <div
                                  key={p}
                                  className={`w-6 h-6 flex items-center justify-center rounded-full border-2 border-white ring-1 ring-slate-100 ${config.color.dot} text-white shadow-sm z-[${10 - idx}] relative group/badge`}
                                  title={config.name}
                                >
                                  <span className="text-[8px] font-bold uppercase">{config.name.substring(0, 1)}</span>
                                </div>
                              );
                            })
                          ) : (
                            <span className="text-[10px] bg-slate-100 text-slate-500 px-1.5 py-0.5 rounded font-bold uppercase">DONE</span>
                          )}
                        </div>
                      </NavLink>
                    </li>
                  );
                })}
              </ul>
            </div>
          )}
        </div>
      </aside>

      {!sidebarOpen && (
        <button
          onClick={toggleSidebar}
          className="absolute left-4 top-4 z-50 p-2 bg-white shadow-md border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors"
          title="Toggle Sidebar"
        >
          <AudioLines className="text-primary w-5 h-5" />
        </button>
      )}

      <main className="flex-1 flex flex-col relative overflow-hidden bg-white">
        <Routes>
          <Route path="/" element={
            <div className="flex-1 flex flex-col items-center justify-center text-slate-400">
              <AudioLines className="w-16 h-16 mb-4 opacity-50" />
              <p>Select a case to begin</p>
            </div>
          } />
          <Route path="/case/:id" element={
            <CaseDetail
              onEvalComplete={loadCases}
              processingCases={processingCases}
              startProcessing={startProcessing}
              endProcessing={endProcessing}
              getSelection={getSelection}
              setSelectionForCase={setSelectionForCase}
              initSelection={initSelection}
            />
          } />
        </Routes>
      </main>
    </div>
  );
}

interface CaseDetailProps {
  onEvalComplete: () => void;
  processingCases: Set<string>;
  startProcessing: (id: string) => void;
  endProcessing: (id: string) => void;
  getSelection: (id: string) => Record<string, boolean>;
  setSelectionForCase: (id: string, newVal: Record<string, boolean>) => void;
  initSelection: (data: LoadingData) => Record<string, boolean>;
}

function CaseDetail({ onEvalComplete, processingCases, startProcessing, endProcessing, getSelection, setSelectionForCase, initSelection }: CaseDetailProps) {
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
    const existingResultsToKeep: Record<string, EvalResult> = {};

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

interface EvalReportViewProps {
  kase: Case;
  selectedProviders: Record<string, boolean>;
  onToggleProvider: (provider: string) => void;
  onSelectAll: () => void;
  onDeselectAll: () => void;
  onSelectDefault: () => void;
  getDefaultSelection: () => Record<string, boolean>;
}

function EvalReportView({ kase, selectedProviders, onToggleProvider, onSelectAll, onDeselectAll, onSelectDefault, getDefaultSelection }: EvalReportViewProps) {
  const evalResults = kase.eval_report?.eval_results || {};
  const hasAI = Object.keys(evalResults).length > 0;

  const providers = Array.from(new Set([
    ...Object.keys(evalResults),
    ...Object.keys(kase.transcripts || {})
  ]));
  const sortedPerformers = Object.entries(evalResults).sort((a, b) => b[1].score - a[1].score);

  const [diffModes, setDiffModes] = useState<Record<string, 'eval' | 'drift' | 'gap'>>({});
  const [sortBy, setSortBy] = useState<'score' | 'name'>('score');

  const sortedProviders = [...providers].sort((a, b) => {
    if (sortBy === 'score') {
      const scoreA = evalResults[a]?.score ?? -1;
      const scoreB = evalResults[b]?.score ?? -1;
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
    <div className="flex flex-col h-full overflow-hidden">
      <div className="bg-white px-8 flex flex-col shrink-0">
        {/* Case ID Header */}
        <div className="py-3 flex items-center justify-between">
          <div className="flex items-center gap-2 pl-8">
            <span className="text-sm font-mono font-medium text-slate-700 select-all">{kase.id}</span>
            <button
              onClick={() => {
                navigator.clipboard.writeText(kase.id);
              }}
              className="p-1 hover:bg-slate-100 rounded text-slate-400 hover:text-primary transition-colors"
              title="Copy Case ID"
            >
              <Copy size={12} />
            </button>
          </div>
        </div>

        {/* Persistent Axis Separator */}
        <div className="relative h-px bg-slate-200">
          {hasAI && (
            <div className="absolute inset-0">
              <div className="relative w-full h-full">
                {/* Score Dots with Tooltips */}
                {sortedPerformers.map(([p, res]) => {
                  const score = res.score * 100;
                  const config = getASRProviderConfig(p);
                  const isNearLeft = score < 15;
                  const isNearRight = score > 85;
                  let tooltipPosition = 'left-1/2 -translate-x-1/2';
                  if (isNearLeft) tooltipPosition = 'left-0';
                  else if (isNearRight) tooltipPosition = 'right-0';
                  return (
                    <div
                      key={p}
                      className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 cursor-pointer group"
                      style={{ left: `${score}%` }}
                    >
                      <div className={`w-3 h-3 rounded-full ${config.color.dot} border-2 border-white shadow-sm hover:scale-125 transition-transform`} />
                      <div className={`absolute bottom-full ${tooltipPosition} mb-2 px-2 py-1 bg-slate-800 text-white text-xs rounded whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-30`}>
                        {config.name}: {Math.round(score)}
                      </div>
                    </div>
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
          const score = aiRes ? Math.round(aiRes.score * 100) : null;
          const config = getASRProviderConfig(p);
          const { color, name } = config;
          const isSelected = !!selectedProviders?.[p];
          const stale = isStale(p);
          const mode = diffModes[p] || 'eval';

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
                {stale ? (
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
                  <div className={`text-3xl font-bold mt-1 ${scoreColorClass}`}>
                    {score !== null ? score : <span className="text-slate-300 text-lg">—</span>}
                  </div>
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
                    navigator.clipboard.writeText(kase.transcripts[p] || "");
                  }}
                  title="Copy original transcript"
                >
                  <Copy size={14} />
                </button>
              </div>

              {/* Column 3: Analysis */}
              <div className="text-xs text-slate-400">
                {aiRes?.summary ? (
                  <ul className="space-y-1.5">
                    {aiRes.summary.map((point, i) => (
                      <li key={i} className="leading-snug">
                        {point}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <span className="text-slate-300 text-lg">—</span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div >
  );
};

const formatTime = (t: number) => {
  const m = Math.floor(t / 60);
  const s = Math.floor(t % 60);
  return `${m}:${s < 10 ? '0' : ''}${s}`;
};

function App() {
  return (
    <BrowserRouter>
      <Layout />
    </BrowserRouter>
  );
}

export default App;
