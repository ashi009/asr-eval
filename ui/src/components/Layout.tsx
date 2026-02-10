import { useState, useEffect } from 'react';
import { Routes, Route, NavLink } from 'react-router-dom';
import { AudioLines, Search, Loader2, BarChart3, AlertTriangle } from 'lucide-react';
import { isProviderEnabled, setEnabledProviders } from '../config';
import { Case } from '../types';

import { CaseDetail } from './CaseDetail';

import { StatsDashboard } from './StatsDashboard';


export function Layout() {
  const [cases, setCases] = useState<Case[]>([]);
  const [search, setSearch] = useState("");
  const [processingCases, setProcessingCases] = useState<Set<string>>(new Set());
  const [caseSelections, setCaseSelections] = useState<Record<string, Record<string, boolean>>>({});

  const initSelection = (data: Case) => {
    const initialSelection: Record<string, boolean> = {};

    const evaluations = data.report_v2?.evaluations;
    const hasEvaluations = evaluations && Object.keys(evaluations).length > 0;

    const providerKeys = new Set([
      ...Object.keys(data.transcripts || {}),
      ...(hasEvaluations ? Object.keys(evaluations || {}) : [])
    ]);

    providerKeys.forEach(provider => {
      const isEnabled = isProviderEnabled(provider);
      const currentTranscript = data.transcripts?.[provider];

      let hasResult = false;
      let isStale = false;

      if (hasEvaluations && evaluations?.[provider]) {
        hasResult = true;
        const res = evaluations[provider];
        // stale if transcript in result differs from current
        isStale = !!res.transcript && currentTranscript !== res.transcript;
      }

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
  const [statsOpen, setStatsOpen] = useState(false);

  useEffect(() => {
    fetch('/api/config')
      .then(res => res.json())
      .then(data => {
        setLlmModel(data.llm_model);
        if (data.enabled_providers) {
          setEnabledProviders(data.enabled_providers);
        }
      })
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
                <button
                  onClick={() => setStatsOpen(true)}
                  className="p-1 rounded hover:bg-slate-100 text-slate-400 hover:text-primary transition-colors"
                  title="View Statistics"
                >
                  <BarChart3 className="w-3.5 h-3.5" />
                </button>
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
                        {c.questionable_gt && <AlertTriangle className="w-3 h-3 text-amber-500 shrink-0" />}
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
                          {c.questionable_gt && <AlertTriangle className="w-3 h-3 text-amber-500 shrink-0" />}
                          {processingCases.has(c.id) && <Loader2 className="w-3 h-3 animate-spin text-primary shrink-0" />}
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

      <StatsDashboard cases={cases} isOpen={statsOpen} onClose={() => setStatsOpen(false)} />
    </div>
  );
}
