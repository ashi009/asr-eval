import { useState, useEffect, useRef } from 'react';
import { BrowserRouter, Routes, Route, NavLink, useParams, useNavigate, Navigate } from 'react-router-dom';
import { Play, Pause, Search, Award, Check, AlertCircle, Volume2, AudioLines, Loader2 } from 'lucide-react';
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
            <span key={index} className="bg-green-100 text-green-700 font-medium px-0.5 rounded mx-0.5 animate-in fade-in duration-300">
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

  // Shared state for optimistic updates
  const updateCaseLocal = (id, updates) => {
    setCases(prev => prev.map(c => c.id === id ? { ...c, ...updates } : c));
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
            />
          } />
        </Routes>
      </main>
    </div>
  );
}

function CaseDetail({ onEvalComplete, processingCases, startProcessing, endProcessing }) {
  const { id } = useParams();
  const idRef = useRef(id); // Track current ID to prevent stale updates
  const [currentCase, setCurrentCase] = useState(null);
  const [loading, setLoading] = useState(true);
  const [selectedServices, setSelectedServices] = useState({});
  const [isInputExpanded, setIsInputExpanded] = useState(true);

  // Keep ref in sync
  const mounted = useRef(true);
  useEffect(() => {
    idRef.current = id;
    mounted.current = true;
    return () => { mounted.current = false; };
  }, [id]);

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
          setCurrentCase(data);

          // Initialize selected services based on config and available results
          const initialSelection = {};
          if (data && data.results) {
            Object.keys(data.results).forEach(service => {
              const config = getServiceConfig(service);
              // Default to config.enabled, fallback to true if undefined
              initialSelection[service] = config.enabled !== false;
            });
          }
          setSelectedServices(initialSelection);

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
        if (mounted.current) setLoading(false);
      }
    };
    if (id) fetchCase();
  }, [id]);

  // Reset player when case changes
  useEffect(() => {
    if (currentCase && audioRef.current) {
      audioRef.current.src = `/audio/${currentCase.id}.flac`;
      audioRef.current.load();
      setIsPlaying(false);
      setCurrentTime(0);
      setDuration(0);
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
        updateCaseLocal({ ai_results: aiResults });
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
      <div className="flex-1 overflow-y-auto px-8 pb-8 space-y-8">
        <ResultsView
          kase={currentCase}
          selectedServices={selectedServices}
          onToggleService={toggleServiceSelection}
        />
      </div>

      {/* Ground Truth Footer */}
      <div className="bg-white border-t border-slate-200 p-4 shrink-0 z-10 transition-all duration-300">
        <div className="max-w-5xl mx-auto">
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
          </div>
        </div>
      </div>
    </>
  );
}

function ResultsView({ kase, selectedServices, onToggleService }) {
  const hasAI = kase.ai_results && Object.keys(kase.ai_results).length > 0;
  const providers = Object.keys(kase.results).sort();
  const sortedPerformers = Object.entries(kase.ai_results || {}).sort((a, b) => a[1].score - b[1].score);

  const scrollToProvider = (p) => {
    const el = document.getElementById(`panel-${p}`);
    if (el) el.scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <div className={`${!hasAI ? 'pt-8' : ''}`}>
      {hasAI && (
        <div className="sticky top-0 z-40 -mx-8 px-8 pt-6 pb-2 bg-slate-50/90 backdrop-blur-xl shadow-sm transition-all mb-6">
          <div className="max-w-5xl mx-auto w-full pb-4 px-6">
            <h2 className="text-sm font-bold mb-8 text-slate-800">Performance Overview</h2>
            <div className="relative h-1.5 bg-slate-200/50 rounded-full mt-12 mb-16 mx-0">
              {/* Axis Markers */}
              <div className="absolute top-1/2 -translate-y-1/2 w-full h-full pointer-events-none">
                {[0, 25, 50, 75, 100].map((val) => (
                  <div
                    key={val}
                    className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 group"
                    style={{ left: `${val}%` }}
                  >
                    <div className="w-1 h-1 bg-slate-300 rounded-full" />
                    <span className="absolute top-4 left-1/2 -translate-x-1/2 text-[9px] font-medium text-slate-400 font-mono mt-1">
                      {val}
                    </span>
                  </div>
                ))}
              </div>

              {sortedPerformers.map(([p, res], idx) => {
                const score = res.score * 100;
                const config = getServiceConfig(p);

                // 4-cycle positioning to handle overlaps
                const position = idx % 4;
                let labelClass = '';
                let tickClass = '';

                switch (position) {
                  case 0: // Top Near
                    labelClass = '-top-7 mb-2';
                    tickClass = 'absolute left-1/2 -translate-x-1/2 w-px h-3 bg-slate-300 -bottom-3';
                    break;
                  case 1: // Bottom Near
                    labelClass = '-bottom-7 mt-2';
                    tickClass = 'absolute left-1/2 -translate-x-1/2 w-px h-3 bg-slate-300 -top-3';
                    break;
                  case 2: // Top Far
                    labelClass = '-top-12 mb-2';
                    tickClass = 'absolute left-1/2 -translate-x-1/2 w-px h-8 bg-slate-300 -bottom-8';
                    break;
                  case 3: // Bottom Far
                    labelClass = '-bottom-12 mt-2';
                    tickClass = 'absolute left-1/2 -translate-x-1/2 w-px h-8 bg-slate-300 -top-8';
                    break;
                }

                return (
                  <div
                    key={p}
                    className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 group cursor-pointer hover:z-50"
                    style={{ left: `${score}%` }}
                    onClick={() => scrollToProvider(p)}
                  >
                    <div className={`w-3 h-3 rounded-full border-2 border-white ${config.color.dot} shadow-sm relative z-30 transition-transform group-hover:scale-125`} />
                    <div
                      className={`absolute left-1/2 -translate-x-1/2 px-2 py-1 bg-white/80 backdrop-blur-sm shadow-sm border border-white/50 rounded text-[10px] font-bold whitespace-nowrap z-20 flex flex-col items-center transition-all group-hover:scale-110 group-hover:bg-white
                        ${labelClass}
                      `}
                    >
                      <span className="text-slate-700 font-bold uppercase tracking-wider text-[10px]">{config.name}</span>
                      <div className={tickClass} />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}

      <div className="max-w-5xl mx-auto space-y-4">
        {providers.map((p) => {
          const aiRes = kase.ai_results?.[p];
          const score = aiRes ? Math.round(aiRes.score * 100) : null;
          const config = getServiceConfig(p);
          const { color, name } = config;
          const isSelected = !!selectedServices?.[p];

          return (
            <div
              id={`panel-${p}`}
              key={p}
              className={`scroll-mt-96 bg-white border rounded-lg overflow-hidden shadow-sm flex flex-col md:flex-row transition-colors
                ${isSelected ? color.border : 'border-slate-200'}
              `}
            >
              {/* Left Column: Header + Transcript */}
              <div className="flex-1 flex flex-col min-w-0 border-r border-slate-100">
                <div className={`px-4 py-3 border-b flex justify-between items-center select-none cursor-pointer transition-colors
                  ${isSelected ? `${color.ring} ${color.border}` : 'bg-slate-50 border-slate-200'}
                `}
                  onClick={() => onToggleService && onToggleService(p)}
                >
                  <div className="flex items-center gap-3">
                    <div className={`relative flex items-center justify-center w-5 h-5 rounded border transition-all shadow-sm
                       ${isSelected ? `${color.dot} border-transparent text-white` : 'bg-white border-slate-300 text-transparent group-hover:border-slate-400'}
                     `}>
                      <Check size={12} strokeWidth={4} className={`transform transition-transform duration-200 ${isSelected ? 'scale-100' : 'scale-0'}`} />
                    </div>
                    <h3 className={`text-sm font-bold uppercase tracking-wide ${isSelected ? color.text : 'text-slate-500'}`}>{name}</h3>
                  </div>
                </div>
                <div className="p-4 text-sm leading-relaxed text-slate-700 min-h-[100px] flex-1">
                  {renderDiff(kase.results[p], aiRes?.revised_transcript)}
                </div>
              </div>

              {/* Right Column: Eval or Placeholder */}
              <div className="w-full md:w-80 shrink-0 bg-slate-50/50 flex flex-col">
                {aiRes ? (
                  <div className="p-4 flex flex-col h-full">
                    <div className="flex items-center justify-between mb-2">
                      <div className="text-3xl font-bold">{score}<span className="text-sm text-slate-400 font-normal">/100</span></div>
                    </div>
                    <h4 className="text-[10px] uppercase font-bold text-slate-400 mb-2">Analysis</h4>
                    <ul className="space-y-1.5 flex-1">
                      {aiRes.summary?.map((point, i) => (
                        <li key={i} className="text-xs text-slate-600 flex gap-2 leading-snug">
                          <AlertCircle size={12} className="text-slate-400 shrink-0 mt-0.5" />
                          <span>{point}</span>
                        </li>
                      ))}
                      {!aiRes.summary && <li className="text-xs text-slate-400 italic">No summary provided</li>}
                    </ul>
                  </div>
                ) : (
                  <div className="p-4 flex flex-col items-center justify-center h-full text-slate-400">
                    <span className="text-xs italic">No AI Eval Result</span>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
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
