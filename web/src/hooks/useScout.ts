import { useState, useEffect, useCallback } from 'react';
import type { ScoutReport } from '../lib/types.ts';

type RefreshPhase = null | 'syncing' | 'sweeping' | 'loading';

async function callTool(tool: string, input: Record<string, unknown> = {}) {
  const resp = await fetch(`/api/tools/scout__${tool}/execute`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ input }),
  });
  return resp.json();
}

export function useScout() {
  const [report, setReport] = useState<ScoutReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [refreshPhase, setRefreshPhase] = useState<RefreshPhase>(null);

  const fetchReport = useCallback(async () => {
    setLoading(true);
    try {
      const data = await callTool('report', {});
      if (data.structured_json) {
        setReport(JSON.parse(data.structured_json));
      }
    } catch (err) {
      console.error('[scout] fetch report failed:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshReport = useCallback(async () => {
    setRefreshPhase('syncing');
    try {
      // 1. Sync all platforms.
      await callTool('sync', { platforms: ['all'] });

      // 2. Sweep all platforms with profile-derived keywords.
      setRefreshPhase('sweeping');
      await callTool('sweep', { platforms: ['all'], keywords: [] });

      // 3. Fetch updated report.
      setRefreshPhase('loading');
      await fetchReport();
    } catch (err) {
      console.error('[scout] refresh failed:', err);
    } finally {
      setRefreshPhase(null);
    }
  }, [fetchReport]);

  // Load report on mount.
  useEffect(() => {
    fetchReport();
  }, [fetchReport]);

  const refreshing = refreshPhase !== null;

  return { report, loading, refreshing, refreshPhase, refreshReport };
}
