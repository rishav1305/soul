import { useState, useEffect, useCallback } from 'react';
import type { TutorMockSession } from '../lib/types';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

interface UseMockSessionReturn {
  session: TutorMockSession | null;
  loading: boolean;
  error: string | null;
  refresh: () => void;
}

export function useMockSession(sessionId: string): UseMockSessionReturn {
  const [session, setSession] = useState<TutorMockSession | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    setLoading(true);
    api.get<TutorMockSession>(`/api/tutor/mocks/${sessionId}`)
      .then(data => { setSession(data); setError(null); })
      .catch(err => { reportError('useMockSession.load', err); setError(err instanceof Error ? err.message : String(err)); })
      .finally(() => setLoading(false));
  }, [sessionId]);

  useEffect(() => {
    refresh();
    reportUsage('mock.view', { sessionId });
  }, [refresh, sessionId]);

  return { session, loading, error, refresh };
}
