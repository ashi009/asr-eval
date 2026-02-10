import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import {
  Case, Config,
  UpdateContextRequest, GenerateContextRequest, EvaluateRequest,
  EvalContext, EvalReport
} from './types';

async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json();
}

const workspaceClient = {
  fetchConfig: async (): Promise<Config> => {
    const res = await fetch('/api/config');
    return handleResponse<Config>(res);
  },

  listCases: async (): Promise<Case[]> => {
    const res = await fetch('/api/cases');
    return handleResponse<Case[]>(res);
  },

  getCase: async (id: string): Promise<Case> => {
    const res = await fetch(`/api/cases/${id}`);
    return handleResponse<Case>(res);
  },

  updateContext: async (req: UpdateContextRequest): Promise<Case> => {
    const res = await fetch(`/api/cases/${req.id}:updateContext`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req)
    });
    return handleResponse<Case>(res);
  },

  generateContext: async (req: GenerateContextRequest, signal?: AbortSignal): Promise<EvalContext> => {
    const res = await fetch(`/api/cases/${req.id}:generateContext`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
      signal
    });
    return handleResponse<EvalContext>(res);
  },

  evaluateCase: async (req: EvaluateRequest): Promise<EvalReport> => {
    const res = await fetch(`/api/cases/${req.id}:evaluate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req)
    });
    return handleResponse<EvalReport>(res);
  },
};

interface WorkspaceState {
  cases: Case[];
  config: Config | null;
  loading: boolean;
  error: string | null;
  refreshCases: () => Promise<void>;
  updateContext: (req: UpdateContextRequest) => Promise<Case>;
  generateContext: (req: GenerateContextRequest, signal?: AbortSignal) => Promise<EvalContext>;
  evaluateCase: (req: EvaluateRequest) => Promise<EvalReport>;
}

const WorkspaceContext = createContext<WorkspaceState | undefined>(undefined);

export const WorkspaceProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [cases, setCases] = useState<Case[]>([]);
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refreshCases = useCallback(async () => {
    try {
      const data = await workspaceClient.listCases();
      // data.sort((a, b) => a.id.localeCompare(b.id)); // sort not strictly needed if backend sorts, but good safety
      setCases(data);
    } catch (err: any) {
      setError(err.message);
    }
  }, []);

  const updateContext = useCallback(async (req: UpdateContextRequest) => {
    const updatedCase = await workspaceClient.updateContext(req);
    setCases(prev => prev.map(c => c.id === updatedCase.id ? { ...c, ...updatedCase } : c));
    return updatedCase;
  }, []);

  const generateContext = useCallback(async (req: GenerateContextRequest, signal?: AbortSignal) => {
    return workspaceClient.generateContext(req, signal);
  }, []);

  const evaluateCase = useCallback(async (req: EvaluateRequest) => {
    return workspaceClient.evaluateCase(req);
  }, []);

  useEffect(() => {
    const init = async () => {
      setLoading(true);
      try {
        const [configData, casesData] = await Promise.all([
          workspaceClient.fetchConfig(),
          workspaceClient.listCases()
        ]);
        setConfig(configData);
        casesData.sort((a, b) => a.id.localeCompare(b.id)); // ensure initial sort
        setCases(casesData);
      } catch (err: any) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    init();
  }, []);

  return (
    <WorkspaceContext.Provider value={{
      cases, config, loading, error, refreshCases,
      updateContext, generateContext, evaluateCase
    }}>
      {children}
    </WorkspaceContext.Provider>
  );
};

export const useWorkspace = () => {
  const context = useContext(WorkspaceContext);
  if (!context) {
    throw new Error('useWorkspace must be used within a WorkspaceProvider');
  }
  return context;
};

// Helper hook for single case
export const useCase = (id: string | undefined) => {
  const [currentCase, setCurrentCase] = useState<Case | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadCase = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const data = await workspaceClient.getCase(id);
      setCurrentCase(data);
      setError(null);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    loadCase();
  }, [loadCase]);

  return { currentCase, loading, error, refresh: loadCase, setCurrentCase };
};
