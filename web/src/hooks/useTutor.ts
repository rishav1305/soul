import { useState, useEffect, useCallback } from 'react';
import type { TutorDashboard, TutorTopic, TutorAnalytics, TutorMockSession } from '../lib/types';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

type TutorTab = 'dashboard' | 'analytics' | 'topics' | 'mocks' | 'guide';

interface UseTutorReturn {
  dashboard: TutorDashboard | null;
  topics: TutorTopic[];
  analytics: TutorAnalytics | null;
  mocks: TutorMockSession[];
  loading: boolean;
  error: string | null;
  activeTab: TutorTab;
  moduleFilter: string;
  setActiveTab: (tab: TutorTab) => void;
  setModuleFilter: (module: string) => void;
  refresh: () => void;
}

export function useTutor(): UseTutorReturn {
  const [dashboard, setDashboard] = useState<TutorDashboard | null>(null);
  const [topics, setTopics] = useState<TutorTopic[]>([]);
  const [analytics, setAnalytics] = useState<TutorAnalytics | null>(null);
  const [mocks, setMocks] = useState<TutorMockSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TutorTab>('dashboard');
  const [moduleFilter, setModuleFilter] = useState('');

  const fetchData = useCallback(async (tab: TutorTab, module: string) => {
    setLoading(true);
    setError(null);
    try {
      switch (tab) {
        case 'dashboard': {
          const data = await api.get<TutorDashboard>('/api/tutor/dashboard');
          setDashboard(data);
          break;
        }
        case 'analytics': {
          const data = await api.get<TutorAnalytics>('/api/tutor/analytics');
          setAnalytics(data);
          break;
        }
        case 'topics': {
          const url = module ? `/api/tutor/topics?module=${module}` : '/api/tutor/topics';
          const data = await api.get<TutorTopic[]>(url);
          setTopics(data ?? []);
          break;
        }
        case 'mocks': {
          const data = await api.get<TutorMockSession[]>('/api/tutor/mocks');
          setMocks(data ?? []);
          break;
        }
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useTutor.fetch', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData(activeTab, moduleFilter);
  }, [activeTab, moduleFilter, fetchData]);

  const handleSetTab = useCallback((tab: TutorTab) => {
    setActiveTab(tab);
    reportUsage('tutor.tab', { tab });
  }, []);

  const refresh = useCallback(() => {
    fetchData(activeTab, moduleFilter);
  }, [activeTab, moduleFilter, fetchData]);

  return { dashboard, topics, analytics, mocks, loading, error, activeTab, moduleFilter, setActiveTab: handleSetTab, setModuleFilter, refresh };
}
