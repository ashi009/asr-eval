import { useState, useEffect, useRef } from 'react';
import { BrowserRouter, Routes, Route, NavLink, useParams, useNavigate, Navigate } from 'react-router-dom';
import { Play, Pause, Search, Award, Check, AlertCircle, Volume2, AudioLines, Loader2, Copy, AlertTriangle, Minus, ArrowUpAZ, ArrowDown01 } from 'lucide-react';
import { getServiceConfig } from './config';
import { smartDiff } from './diffUtils';

/* --- Helper: Diff Render --- */
const renderDiff = (original, revised) => {
  if (!revised) return original;

  const diffs = smartDiff(original, revised);

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

function Layout() {
  const [cases, setCases] = useState([]);
  const [search, setSearch] = useState("");
  const [processingCases, setProcessingCases] = useState(new Set()); // Track evaluating case IDs
  const [caseSelections, setCaseSelections] = useState({}); // Lifted selection state

  // Shared state for optimistic updates
  const updateCaseLocal = (id, updates) => {
    setCases(prev => prev.map(c => c.id === id ? { ...c, ...updates } : c));
  };

  // Selection helper
  const initSelection = (data) => {
    const initialSelection = {};
    if (data.results) {
      Object.keys(data.results).forEach(service => {
        const config = getServiceConfig(service);
        // Auto-select if enabled AND not already evaluated
        const isEnabled = config.enabled !== false;
        const hasResult = data.ai_results && data.ai_results[service];

        // Also check if transcript is stale
        let isStale = false;
        if (hasResult && data.evaluated_transcripts && data.results[service]) {
          if (data.evaluated_transcripts[service] !== data.results[service]) {
            isStale = true;
          }
        }

        initialSelection[service] = isEnabled && (!hasResult || isStale);
      });
    }
    return initialSelection;
  };

  const getSelection = (id) => caseSelections[id];
  const updateSelection = (id, loadingData) => {
    // If we are loading fresh data and have no selection, init it.
    // OR if we are forcing a reset (loadingData passed with cleared results)
    if (!caseSelections[id] || loadingData) {
      setCaseSelections(prev => ({
        ...prev,
        [id]: initSelection(loadingData || {}) // This might be buggy if called without data first, but usage below handles it.
      }));
    }
  };

  // Direct setter for manual toggles
  const setSelectionForCase = (id, newVal) => {
    setCaseSelections(prev => ({ ...prev, [id]: newVal }));
  };

  // Eval status handlers
  const startProcessing = (id) => {
    setProcessingCases(prev => new Set(prev).add(id));
  };

  const endProcessing = (id) => {
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
      // data is just [{id: '...', has_ai: bool}, ...]
      data.sort((a, b) => a.id.localeCompare(b.id));
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

  // Sidebar State
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [isOverlay, setIsOverlay] = useState(false);

  useEffect(() => {
    const handleResize = () => {
      const overlay = window.innerWidth < 1280;
      setIsOverlay(overlay);

      // Auto-retract only when switching to overlay mode initially?
      // For now, keeping simple logic: if overlay, default to closed unless manually toggled?
      // Actually, better UX: if window shrinks to overlay, close it.
      if (overlay) {
        setSidebarOpen(false);
      } else {
        setSidebarOpen(true);
      }
    };

    // Initial check
    handleResize();

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const toggleSidebar = () => setSidebarOpen(!sidebarOpen);

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
            <div className="flex items-center gap-2">
              <AudioLines className="text-primary" />
              <h1 className="font-bold">ASR Eval Pro</h1>
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
                        className={({ isActive }) => `block w-full text-left px-3 py-2 rounded text-sm font-mono flex justify-between items-center group
                          ${isActive ? 'bg-white shadow ring-1 ring-slate-200 border-l-2 border-primary' : 'hover:bg-slate-200/50 text-slate-600 border-l-2 border-transparent'}
                        `}
                      >
                        <div className="flex items-center gap-2 min-w-0 flex-1">
                          <span className="truncate block">{c.id}</span>
                          {processingCases.has(c.id) && <Loader2 className="w-3 h-3 animate-spin text-primary shrink-0" />}
                        </div>
                        <div className="flex items-center -space-x-1 shrink-0 ml-2">
                          {c.best_performers && c.best_performers.length > 0 ? (
                            c.best_performers.map((p, idx) => {
                              const config = getServiceConfig(p);
                              return (
                                <div
                                  key={p}
                                  className={`w-6 h-6 flex items-center justify-center rounded-full border-2 border-white ring-1 ring-slate-100 ${config.color.dot} text-white shadow-sm z-[${10 - idx}] relative group/badge`}
                                  title={config.name}
                                >
                                  <span className="text-[8px] font-bold uppercase">{p.substring(0, 1)}</span>
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

      {/* Sidebar Toggle Button (Visible when sidebar is closed) */}
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

function CaseDetail({ onEvalComplete, processingCases, startProcessing, endProcessing, getSelection, setSelectionForCase, initSelection }) {
  const { id } = useParams();
  const idRef = useRef(id); // Track current ID to prevent stale updates
  const [currentCase, setCurrentCase] = useState(null);
  const [loading, setLoading] = useState(true);
  const [selectedServices, setSelectedServices] = useState({});
  // Use lifted selection state helpers
  // const selectedServices = getSelection(id) || {}; // REMOVED
  // const setSelectedServices = (newVal) => setSelectionForCase(id, newVal); // REMOVED

  const [isInputExpanded, setIsInputExpanded] = useState(true);

  // Keep ref in sync
  const mounted = useRef(true);
  useEffect(() => {
    idRef.current = id;
    mounted.current = true;
    return () => { mounted.current = false; };
  }, [id]);

  // Scroll container ref
  const scrollContainerRef = useRef(null);

  // Player
  const audioRef = useRef(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);

  useEffect(() => {
    const fetchCase = async () => {
      setLoading(true);
      try {
        const res = await fetch(`/api/case?id=${id}`);
        if (!res.ok) throw new Error("Failed to load case");
        const data = await res.json();
        if (mounted.current && idRef.current === id) {
          // If backend doesn't provide specific evaluated_ground_truth,
          // assume the loaded GT is the baseline for existing results.
          if (!data.evaluated_ground_truth && data.evaluation?.ground_truth) {
            data.evaluated_ground_truth = data.evaluation.ground_truth;
          }

          setCurrentCase(data);

          // Initialize selected services
          // Rule: If currently processing, restore persisted selection.
          // Otherwise, use default rules (stale/unevaluated).
          if (processingCases.has(id)) {
            const persisted = getSelection(id);
            if (persisted) {
              setSelectedServices(persisted);
            } else {
              // Fallback if no persisted state found (shouldn't happen if runEval saves it)
              setSelectedServices(initSelection(data));
            }
          } else {
            // Not processing, always reset to default rules on fresh load
            setSelectedServices(initSelection(data));
          }

          // Auto-collapse if ground truth exists
          if (data.evaluation?.ground_truth?.trim()) {
            setIsInputExpanded(false);
          } else {
            setIsInputExpanded(true);
          }
        }
      } catch (e) {
        console.error(e);
      } finally {
        if (mounted.current && idRef.current === id) setLoading(false);
      }
    };
    if (id) fetchCase();
  }, [id]);

  // Reset player and scroll to top when case changes
  useEffect(() => {
    if (currentCase && audioRef.current) {
      audioRef.current.src = `/audio/${currentCase.id}.flac`;
      audioRef.current.load();
      setIsPlaying(false);
      setCurrentTime(0);
      setDuration(0);
    }
    // Scroll to top when case changes
    if (scrollContainerRef.current) {
      scrollContainerRef.current.scrollTop = 0;
    }
  }, [currentCase?.id]);

  const togglePlay = () => {
    if (audioRef.current.paused) {
      audioRef.current.play();
      setIsPlaying(true);
    } else {
      audioRef.current.pause();
      setIsPlaying(false);
    }
  };

  const updateCaseLocal = (updates) => {
    if (mounted.current) {
      setCurrentCase(prev => ({ ...prev, ...updates }));
    }
  };

  const toggleServiceSelection = (service) => {
    setSelectedServices(prev => ({
      ...prev,
      [service]: !prev[service]
    }));
  };

  const runEval = async () => {
    const gt = currentCase.evaluation?.ground_truth || "";
    if (!gt.trim()) return alert("Ground Truth required");

    const evalId = currentCase.id;

    // Filter results based on selection
    const resultsToEval = {};
    const existingResultsToKeep = {};

    Object.keys(currentCase.results).forEach(service => {
      if (selectedServices[service]) {
        resultsToEval[service] = currentCase.results[service];
      } else if (currentCase.ai_results && currentCase.ai_results[service]) {
        // If not selected but has existing result, keep it
        existingResultsToKeep[service] = currentCase.ai_results[service];
      }
    });

    if (Object.keys(resultsToEval).length === 0) {
      return alert("Please select at least one service to evaluate.");
    }

    // Persist selection before starting
    setSelectionForCase(evalId, selectedServices);
    startProcessing(evalId);

    try {
      const res = await fetch('/api/evaluate-llm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: evalId,
          ground_truth: gt,
          results: resultsToEval,
          existing_results: existingResultsToKeep
        })
      });
      if (!res.ok) throw await res.text();
      const aiResults = await res.json();

      // Check against current ID using Ref to avoid stale closure
      if (idRef.current === evalId) {
        const newData = {
          ...currentCase,
          ai_results: aiResults,
          evaluated_ground_truth: gt
        };
        updateCaseLocal(newData);

        // Refresh selection based on new results (should uncheck the just-evaluated ones)
        setSelectedServices(initSelection(newData));
      }
      if (onEvalComplete) onEvalComplete();
    } catch (e) {
      alert("Eval Failed: " + e);
    } finally {
      endProcessing(evalId);
    }
  };

  const isProcessingThisCase = processingCases.has(currentCase?.id);

  if (loading) return <div className="p-8 text-center text-slate-500">Loading case data...</div>;
  if (!currentCase) {
    return <div className="p-8 text-center text-slate-500">Case not found.</div>;
  }

  return (
    <>
      {/* Player Header Removed (Moved to Footer) */}

      {/* Scrollable Content */}
      <div ref={scrollContainerRef} className="flex-1 overflow-y-auto px-8 pb-8 space-y-8">


        <ResultsView
          kase={currentCase}
          selectedServices={selectedServices}
          onToggleService={toggleServiceSelection}
          onSelectAll={() => {
            const allServices = Object.keys(currentCase.results);
            const newSelection = {};
            allServices.forEach(s => newSelection[s] = true);
            setSelectedServices(newSelection);
          }}
          onDeselectAll={() => {
            setSelectedServices({});
          }}
          onSelectDefault={() => {
            setSelectedServices(initSelection(currentCase));
          }}
          getDefaultSelection={() => initSelection(currentCase)}
        />
      </div>

      {/* Ground Truth Footer */}
      <div className="bg-white border-t border-slate-200 px-8 py-4 shrink-0 z-10 transition-all duration-300">
        {/* Playback Controls */}
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
            ref={audioRef}
            onTimeUpdate={e => setCurrentTime(e.target.currentTime)}
            onLoadedMetadata={e => setDuration(e.target.duration)}
            onEnded={() => setIsPlaying(false)}
          />
        </div>

        {/* Ground Truth Section */}
        <div className="flex flex-col gap-2">
          <div className="flex justify-between items-center">
            <div
              className="flex items-center gap-2 cursor-pointer select-none"
              onClick={() => setIsInputExpanded(!isInputExpanded)}
            >
              <label className="text-sm font-bold flex items-center gap-2 cursor-pointer">
                <Check className={`w-4 h-4 transition-colors ${currentCase.evaluation?.ground_truth ? 'text-green-500' : 'text-slate-400'}`} />
                Ground Truth
              </label>
              <span className="text-xs text-slate-400 hover:text-primary transition-colors">
                {isInputExpanded ? '(Click to collapse)' : '(Click to expand)'}
              </span>
            </div>
            <button
              onClick={runEval}
              disabled={isProcessingThisCase}
              className="bg-primary hover:bg-blue-600 disabled:opacity-50 text-white px-3 py-1 rounded text-xs font-bold flex items-center gap-2 transition-colors ml-auto"
            >
              {isProcessingThisCase ? 'Running...' : <><Play size={12} /> Run AI Eval ({Object.values(selectedServices).filter(Boolean).length})</>}
            </button>
          </div>

          {isInputExpanded ? (
            <textarea
              className="w-full h-32 border border-slate-200 rounded p-3 text-sm font-mono focus:ring-1 focus:ring-primary focus:border-primary disabled:bg-slate-50 disabled:text-slate-500 animate-in fade-in zoom-in-95 duration-200"
              placeholder="Enter ground truth..."
              value={currentCase.evaluation?.ground_truth || ""}
              onChange={e => updateCaseLocal({ evaluation: { ...currentCase.evaluation, ground_truth: e.target.value } })}
              disabled={isProcessingThisCase}
              autoFocus
            />
          ) : (
            <div
              className="w-full bg-slate-50 border border-slate-200 rounded p-2 text-sm text-slate-600 font-mono italic cursor-pointer hover:border-slate-300 hover:bg-slate-100 transition-colors truncate"
              onClick={() => setIsInputExpanded(true)}
              title={currentCase.evaluation?.ground_truth}
            >
              {currentCase.evaluation?.ground_truth || "No ground truth provided"}
            </div>
          )}

          {/* GT Mismatch Warning */}
          {currentCase.ai_results && Object.keys(currentCase.ai_results).length > 0 &&
            currentCase.evaluated_ground_truth &&
            currentCase.evaluation?.ground_truth &&
            currentCase.evaluated_ground_truth !== currentCase.evaluation.ground_truth && (
              <div className="mt-1 px-3 py-2 bg-amber-50 border border-amber-200 rounded flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <AlertTriangle size={14} className="text-amber-600" />
                  <span className="text-xs font-medium text-amber-800">Results outdated</span>
                </div>
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => updateCaseLocal({ evaluation: { ...currentCase.evaluation, ground_truth: currentCase.evaluated_ground_truth } })}
                    className="text-xs font-bold text-amber-700 hover:text-amber-900 hover:underline"
                  >
                    Revert GT
                  </button>
                  <div className="w-px h-3 bg-amber-300"></div>
                  <button
                    onClick={async () => {
                      if (!confirm("Are you sure you want to delete all evaluation results for this case?")) return;
                      try {
                        const res = await fetch(`/api/reset-eval?id=${currentCase.id}`, { method: 'POST' });
                        if (!res.ok) throw await res.text();
                        onEvalComplete();

                        const resetData = { ...currentCase, ai_results: {}, evaluated_ground_truth: null };
                        setCurrentCase(resetData);
                        // Force reset selection
                        const newSelection = initSelection(resetData);
                        setSelectedServices(newSelection);
                      } catch (e) {
                        alert("Failed to reset: " + e);
                      }
                    }}
                    className="text-xs font-bold text-amber-700 hover:text-amber-900 hover:underline"
                  >
                    Reset Results
                  </button>
                </div>
              </div>
            )}
        </div>
      </div>
    </>
  );
}

function ResultsView({ kase, selectedServices, onToggleService, onSelectAll, onDeselectAll, onSelectDefault, getDefaultSelection }) {
  const hasAI = kase.ai_results && Object.keys(kase.ai_results).length > 0;
  const providers = Object.keys(kase.results);
  const sortedPerformers = Object.entries(kase.ai_results || {}).sort((a, b) => b[1].score - a[1].score);

  // Sort state: 'score' (default) or 'name'
  const [sortBy, setSortBy] = useState('score');

  // Sort providers by score desc (default) or name
  const sortedProviders = [...providers].sort((a, b) => {
    if (sortBy === 'score') {
      const scoreA = kase.ai_results?.[a]?.score ?? -1;
      const scoreB = kase.ai_results?.[b]?.score ?? -1;
      if (scoreB !== scoreA) return scoreB - scoreA;
      // If same score, sort by name
      return getServiceConfig(a).name.localeCompare(getServiceConfig(b).name);
    } else {
      return getServiceConfig(a).name.localeCompare(getServiceConfig(b).name);
    }
  });

  // Calculate selection state for header checkbox
  const selectedCount = providers.filter(p => selectedServices?.[p]).length;
  const selectionState = selectedCount === 0 ? 'none' : selectedCount === providers.length ? 'all' : 'partial';

  // Get default selection to compare
  const defaultSelection = getDefaultSelection ? getDefaultSelection() : {};
  const defaultSelectedCount = providers.filter(p => defaultSelection[p]).length;
  const defaultEqualsAll = defaultSelectedCount === providers.length;

  // 3-state toggle: none → default → all → none
  // If default equals all, then: none → all → none (skip partial)
  const handleHeaderCheckboxClick = () => {
    if (selectionState === 'none') {
      // From none → apply default
      // IF default is empty, skip to all
      if (defaultSelectedCount > 0 && onSelectDefault) {
        onSelectDefault();
      } else if (onSelectAll) {
        onSelectAll();
      }
    } else if (selectionState === 'partial') {
      // From partial → select all
      if (onSelectAll) onSelectAll();
    } else {
      // From all → deselect all
      if (onDeselectAll) onDeselectAll();
    }
  };

  const isStale = (service) => {
    if (!kase.ai_results?.[service]) return false;
    if (!kase.evaluated_transcripts?.[service]) return false;
    return kase.evaluated_transcripts[service] !== kase.results[service];
  };

  return (
    <div>
      {/* Sticky Header: Perf Distribution + Grid Header */}
      <div className="sticky top-0 z-20 bg-white -mx-8 px-8">
        {/* Performance Distribution */}
        {hasAI && (
          <div className="pt-4 pb-8">
            <h3 className="text-[10px] font-bold uppercase tracking-wider text-slate-400 mb-4">Perf Distribution</h3>
            <div className="relative h-1 bg-slate-200 rounded-full mx-3">
              {/* Axis Markers */}
              {[0, 50, 100].map((val) => (
                <div
                  key={val}
                  className="absolute top-1/2"
                  style={{ left: `${val}%`, transform: `translateX(-50%) translateY(-50%)` }}
                >
                  <span className="absolute top-4 left-1/2 -translate-x-1/2 text-[9px] font-mono text-slate-400">
                    {val}
                  </span>
                </div>
              ))}
              {/* Score Dots with Tooltips */}
              {sortedPerformers.map(([p, res]) => {
                const score = res.score * 100;
                const config = getServiceConfig(p);
                // Viewport-aware tooltip: left edge -> right align, right edge -> left align
                const isNearLeft = score < 15;
                const isNearRight = score > 85;
                let tooltipPosition = 'left-1/2 -translate-x-1/2'; // default: center
                if (isNearLeft) tooltipPosition = 'left-0';
                else if (isNearRight) tooltipPosition = 'right-0';
                return (
                  <div
                    key={p}
                    className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 cursor-pointer group"
                    style={{ left: `${score}%` }}
                  >
                    <div className={`w-3 h-3 rounded-full ${config.color.dot} border-2 border-white shadow-sm`} />
                    <div className={`absolute bottom-full ${tooltipPosition} mb-2 px-2 py-1 bg-slate-800 text-white text-xs rounded whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none`}>
                      {config.name}: {Math.round(score)}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

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
                className="w-5 h-5 rounded-full flex items-center justify-center transition-all shrink-0 bg-slate-400 border-2 border-white shadow-sm cursor-pointer"
              >
                {selectionState === 'all' && <Check size={12} className="text-white" strokeWidth={4} />}
                {selectionState === 'partial' && <Minus size={12} className="text-white" strokeWidth={4} />}
              </button>
            </div>
            {/* Service / Score - clickable text to sort */}
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setSortBy('name')}
                className={`hover:text-slate-600 cursor-pointer ${sortBy === 'name' ? 'text-slate-700' : ''}`}
              >
                SERVICE
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

      {/* Grid Rows */}
      {sortedProviders.map((p) => {
        const aiRes = kase.ai_results?.[p];
        const score = aiRes ? Math.round(aiRes.score * 100) : null;
        const config = getServiceConfig(p);
        const { color, name } = config;
        const isSelected = !!selectedServices?.[p];
        const stale = isStale(p);

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
              onClick={() => onToggleService && onToggleService(p)}
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
                onClick={() => onToggleService && onToggleService(p)}
              >
                <span
                  className="text-xs font-bold uppercase tracking-wide text-slate-700 hover:text-slate-900"
                  style={{ lineHeight: '20px' }}
                >
                  {name}
                </span>
                {stale && (
                  <AlertTriangle
                    size={10}
                    className="text-amber-500 animate-pulse shrink-0"
                    style={{ marginTop: '5px' }}
                  />
                )}
              </div>
              {/* Score - not clickable */}
              <div className={`text-3xl font-bold mt-1 ${scoreColorClass}`}>
                {score !== null ? score : <span className="text-slate-300 text-lg">—</span>}
              </div>
            </div>

            {/* Column 2: Transcript Diff - 20px line height to match */}
            <div
              className="text-sm text-slate-700 relative group pr-8"
              style={{ lineHeight: '20px' }}
            >
              {renderDiff(kase.results[p], aiRes?.revised_transcript)}
              <button
                className="absolute top-0 right-2 p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded transition-all opacity-0 group-hover:opacity-100"
                onClick={(e) => {
                  e.stopPropagation();
                  navigator.clipboard.writeText(kase.results[p]);
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
  );
}

const formatTime = (t) => {
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
