import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

// --- Types ---

export interface ScoutLead {
  id: number;
  title: string;
  company: string;
  type: 'job' | 'freelance' | 'contract' | 'consulting' | 'product-dev';
  source: string;
  stage: string;
  match_score: number;
  compensation: string;
  contact: string;
  location: string;
  notes: string;
  url: string;
  created_at: string;
  updated_at: string;
}

export interface ScoutAnalytics {
  by_type: Record<string, number>;
  by_source: Record<string, number>;
  by_stage: Record<string, number>;
  conversion: { stage: string; count: number; rate: number }[];
  weekly_trend: { week: string; count: number }[];
  total_leads: number;
  active_leads: number;
}

export interface ScoutSweepStatus {
  running: boolean;
  platforms: string[];
  started_at: string;
  progress: number;
  results_found: number;
}

export interface ScoutProfile {
  experience: { title: string; company: string; duration: string; description: string }[];
  projects: { name: string; description: string; url: string }[];
  skills: string[];
  education: { degree: string; institution: string; year: string }[];
  certifications: { name: string; issuer: string; year: string }[];
}

export interface ScoutOptimization {
  id: number;
  type: string;
  field: string;
  current: string;
  suggested: string;
  reason: string;
  status: 'pending' | 'approved' | 'rejected';
}

export interface ScoutAgentRun {
  id: number;
  platform: string;
  mode: string;
  status: 'running' | 'completed' | 'failed';
  created_at: string;
  completed_at: string;
  results_found: number;
  summary: string;
}

export interface ScoutScoredLead {
  id: number;
  title: string;
  company: string;
  type: string;
  stage: string;
  match_score: number;
}

export type ScoutTab = 'pipeline' | 'analytics' | 'actions' | 'profile' | 'intelligence';

export interface UseScoutReturn {
  leads: ScoutLead[];
  analytics: ScoutAnalytics | null;
  sweepStatus: ScoutSweepStatus | null;
  profile: ScoutProfile | null;
  optimizations: ScoutOptimization[];
  agentRuns: ScoutAgentRun[];
  scoredLeads: ScoutScoredLead[];
  loading: boolean;
  error: string | null;
  activeTab: ScoutTab;
  setActiveTab: (tab: ScoutTab) => void;
  refresh: () => void;
  addLead: (data: Partial<ScoutLead>) => Promise<void>;
  updateLead: (id: number, data: Partial<ScoutLead>) => Promise<void>;
  triggerSweep: (platforms: string[]) => Promise<void>;
  syncPlatform: (platform: string) => Promise<void>;
}

export function useScout(): UseScoutReturn {
  const [leads, setLeads] = useState<ScoutLead[]>([]);
  const [analytics, setAnalytics] = useState<ScoutAnalytics | null>(null);
  const [sweepStatus, setSweepStatus] = useState<ScoutSweepStatus | null>(null);
  const [profile, setProfile] = useState<ScoutProfile | null>(null);
  const [optimizations, setOptimizations] = useState<ScoutOptimization[]>([]);
  const [agentRuns, setAgentRuns] = useState<ScoutAgentRun[]>([]);
  const [scoredLeads, setScoredLeads] = useState<ScoutScoredLead[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<ScoutTab>('pipeline');

  const fetchData = useCallback(async (tab: ScoutTab) => {
    setLoading(true);
    setError(null);
    try {
      switch (tab) {
        case 'pipeline': {
          const data = await api.get<ScoutLead[]>('/api/scout/leads');
          setLeads(data ?? []);
          break;
        }
        case 'analytics': {
          const data = await api.get<ScoutAnalytics>('/api/scout/analytics');
          setAnalytics(data);
          break;
        }
        case 'actions': {
          const [sweep, opts] = await Promise.all([
            api.get<ScoutSweepStatus>('/api/scout/sweep/status'),
            api.get<ScoutOptimization[]>('/api/scout/optimizations'),
          ]);
          setSweepStatus(sweep);
          setOptimizations(opts ?? []);
          break;
        }
        case 'profile': {
          const data = await api.get<ScoutProfile>('/api/scout/profile');
          setProfile(data);
          break;
        }
        case 'intelligence': {
          const [scored, runs] = await Promise.all([
            api.get<ScoutScoredLead[]>('/api/scout/leads/scored'),
            api.get<ScoutAgentRun[]>('/api/scout/sweep/status').then(() =>
              // Agent runs come from sweep status; fetch separately if available
              api.get<ScoutAgentRun[]>('/api/scout/leads/scored').catch(() => [] as ScoutAgentRun[])
            ).catch(() => [] as ScoutAgentRun[]),
          ]);
          setScoredLeads(scored ?? []);
          setAgentRuns(runs ?? []);
          break;
        }
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useScout.fetch', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  const handleSetTab = useCallback((tab: ScoutTab) => {
    setActiveTab(tab);
    reportUsage('scout.tab', { tab });
  }, []);

  const refresh = useCallback(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  const addLead = useCallback(async (data: Partial<ScoutLead>) => {
    await api.post<ScoutLead>('/api/scout/leads', data);
    reportUsage('scout.addLead');
    await fetchData('pipeline');
  }, [fetchData]);

  const updateLead = useCallback(async (id: number, data: Partial<ScoutLead>) => {
    await api.patch<ScoutLead>(`/api/scout/leads/${id}`, data);
    reportUsage('scout.updateLead', { id });
    await fetchData(activeTab);
  }, [activeTab, fetchData]);

  const triggerSweep = useCallback(async (platforms: string[]) => {
    await api.post('/api/scout/sweep', { platforms });
    reportUsage('scout.triggerSweep', { platforms: platforms.join(',') });
    await fetchData('actions');
  }, [fetchData]);

  const syncPlatform = useCallback(async (platform: string) => {
    await api.post('/api/scout/sync', { platform });
    reportUsage('scout.syncPlatform', { platform });
    await fetchData('actions');
  }, [fetchData]);

  return {
    leads, analytics, sweepStatus, profile, optimizations, agentRuns, scoredLeads,
    loading, error, activeTab, setActiveTab: handleSetTab, refresh,
    addLead, updateLead, triggerSweep, syncPlatform,
  };
}
