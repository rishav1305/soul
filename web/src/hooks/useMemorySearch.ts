import { useState, useCallback } from 'react';
import type { MemoryResult, MemoryQuery } from '../components/soulgraph/MemorySearch';
import type { MemoryHealthData } from '../components/soulgraph/MemoryHealth';
import { reportError, reportUsage } from '../lib/telemetry';

/**
 * SoulGraph memory API base URL.
 * In production, this points to the SoulGraph FastAPI server.
 * Defaults to localhost:9080 (titan-pc Docker).
 */
const SOULGRAPH_API = 'http://localhost:9080';

interface UseMemorySearchReturn {
  results: MemoryResult[];
  health: MemoryHealthData | null;
  loading: boolean;
  healthLoading: boolean;
  error: string | null;
  healthError: string | null;
  search: (query: MemoryQuery) => Promise<MemoryResult[]>;
  fetchHealth: () => Promise<void>;
}

export function useMemorySearch(): UseMemorySearchReturn {
  const [results, setResults] = useState<MemoryResult[]>([]);
  const [health, setHealth] = useState<MemoryHealthData | null>(null);
  const [loading, setLoading] = useState(false);
  const [healthLoading, setHealthLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [healthError, setHealthError] = useState<string | null>(null);

  const search = useCallback(async (query: MemoryQuery): Promise<MemoryResult[]> => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${SOULGRAPH_API}/memory/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(query),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({ detail: res.statusText }));
        throw new Error(body.detail || `HTTP ${res.status}`);
      }
      const data: MemoryResult[] = await res.json();
      setResults(data);
      reportUsage('memory.search', { collection: query.collection, top_k: query.top_k });
      return data;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useMemorySearch.search', err);
      setError(message);
      return [];
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchHealth = useCallback(async () => {
    setHealthLoading(true);
    setHealthError(null);
    try {
      const res = await fetch(`${SOULGRAPH_API}/memory/health`);
      if (!res.ok) {
        throw new Error(`Health check failed: HTTP ${res.status}`);
      }
      const data: MemoryHealthData = await res.json();
      setHealth(data);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useMemorySearch.health', err);
      setHealthError(message);
    } finally {
      setHealthLoading(false);
    }
  }, []);

  return {
    results,
    health,
    loading,
    healthLoading,
    error,
    healthError,
    search,
    fetchHealth,
  };
}
