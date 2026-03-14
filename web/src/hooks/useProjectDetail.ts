import { useState, useEffect, useCallback } from 'react';
import type { ProjectDetail } from '../lib/types';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

interface UseProjectDetailReturn {
  project: ProjectDetail | null;
  guide: string;
  loading: boolean;
  error: string | null;
  updateProject: (fields: Record<string, unknown>) => Promise<void>;
  updateMilestone: (milestoneId: number, fields: Record<string, unknown>) => Promise<void>;
  recordMetric: (name: string, value: string, unit: string) => Promise<void>;
  updateReadiness: (fields: Record<string, unknown>) => Promise<void>;
  syncPlatform: (platform: string, synced: boolean) => Promise<void>;
  refresh: () => void;
}

export function useProjectDetail(projectId: number): UseProjectDetailReturn {
  const [project, setProject] = useState<ProjectDetail | null>(null);
  const [guide, setGuide] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [detail, guideText] = await Promise.all([
        api.get<ProjectDetail>(`/api/projects/${projectId}`),
        api.get<string>(`/api/projects/${projectId}/guide`).catch(() => ''),
      ]);
      setProject(detail);
      setGuide(guideText || '');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useProjectDetail.fetch', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const updateProject = useCallback(async (fields: Record<string, unknown>) => {
    try {
      await api.patch(`/api/projects/${projectId}`, fields);
      reportUsage('project.update', { projectId });
      await fetchData();
    } catch (err: unknown) {
      reportError('useProjectDetail.updateProject', err);
      throw err;
    }
  }, [projectId, fetchData]);

  const updateMilestone = useCallback(async (milestoneId: number, fields: Record<string, unknown>) => {
    try {
      await api.patch(`/api/projects/${projectId}/milestones/${milestoneId}`, fields);
      reportUsage('project.updateMilestone', { projectId, milestoneId });
      await fetchData();
    } catch (err: unknown) {
      reportError('useProjectDetail.updateMilestone', err);
      throw err;
    }
  }, [projectId, fetchData]);

  const recordMetric = useCallback(async (name: string, value: string, unit: string) => {
    try {
      await api.post(`/api/projects/${projectId}/metrics`, { name, value, unit });
      reportUsage('project.recordMetric', { projectId });
      await fetchData();
    } catch (err: unknown) {
      reportError('useProjectDetail.recordMetric', err);
      throw err;
    }
  }, [projectId, fetchData]);

  const updateReadiness = useCallback(async (fields: Record<string, unknown>) => {
    try {
      await api.post(`/api/projects/${projectId}/readiness`, fields);
      reportUsage('project.updateReadiness', { projectId });
      await fetchData();
    } catch (err: unknown) {
      reportError('useProjectDetail.updateReadiness', err);
      throw err;
    }
  }, [projectId, fetchData]);

  const syncPlatform = useCallback(async (platform: string, synced: boolean) => {
    try {
      await api.post(`/api/projects/${projectId}/syncs`, { platform, synced });
      reportUsage('project.syncPlatform', { projectId, platform });
      await fetchData();
    } catch (err: unknown) {
      reportError('useProjectDetail.syncPlatform', err);
      throw err;
    }
  }, [projectId, fetchData]);

  const refresh = useCallback(() => {
    fetchData();
  }, [fetchData]);

  return { project, guide, loading, error, updateProject, updateMilestone, recordMetric, updateReadiness, syncPlatform, refresh };
}
