import { useState, useEffect, useCallback } from 'react';
import type { ScoutReport } from '../lib/types.ts';

export function useScout() {
  const [report, setReport] = useState<ScoutReport | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchReport = useCallback(async (period = 'today') => {
    setLoading(true);
    try {
      const resp = await fetch('/api/tools/scout__report/execute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ input: { period } }),
      });
      const data = await resp.json();
      if (data.structured_json) {
        setReport(JSON.parse(data.structured_json));
      }
    } catch (err) {
      console.error('[scout] fetch report failed:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchReport();
  }, [fetchReport]);

  return { report, loading, refreshReport: fetchReport };
}
