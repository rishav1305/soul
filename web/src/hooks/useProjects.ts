import { useState, useEffect, useCallback } from 'react';
import type { ProjectDashboard, ProjectKeyword } from '../lib/types';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

type ProjectsTab = 'dashboard' | 'projects' | 'timeline' | 'keywords';

interface UseProjectsReturn {
  dashboard: ProjectDashboard | null;
  keywords: ProjectKeyword[];
  loading: boolean;
  error: string | null;
  activeTab: ProjectsTab;
  setActiveTab: (tab: ProjectsTab) => void;
  refresh: () => void;
}

export function useProjects(): UseProjectsReturn {
  const [dashboard, setDashboard] = useState<ProjectDashboard | null>(null);
  const [keywords, setKeywords] = useState<ProjectKeyword[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<ProjectsTab>('dashboard');

  const fetchData = useCallback(async (tab: ProjectsTab) => {
    setLoading(true);
    setError(null);
    try {
      switch (tab) {
        case 'dashboard':
        case 'projects':
        case 'timeline': {
          const data = await api.get<ProjectDashboard>('/api/projects/dashboard');
          setDashboard(data);
          break;
        }
        case 'keywords': {
          const data = await api.get<ProjectKeyword[]>('/api/projects/keywords');
          setKeywords(data ?? []);
          break;
        }
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useProjects.fetch', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  const handleSetTab = useCallback((tab: ProjectsTab) => {
    setActiveTab(tab);
    reportUsage('projects.tab', { tab });
  }, []);

  const refresh = useCallback(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  return { dashboard, keywords, loading, error, activeTab, setActiveTab: handleSetTab, refresh };
}
