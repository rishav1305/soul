import { useState, useCallback } from 'react';
import type { MemoryResult, MemoryQuery, MemoryQueryResponse } from '../components/soulgraph/MemorySearch';
import type { MemoryHealthData } from '../components/soulgraph/MemoryHealth';
import { reportError, reportUsage } from '../lib/telemetry';

/**
 * SoulGraph memory API base URL.
 * Banner's API on titan-pc (:3030). Per Shuri's confirmed contract (Mar 30).
 * Endpoints: POST /query, GET /health, GET /status
 */
const SOULGRAPH_API = 'http://192.168.0.196:3030';

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
      // Map internal MemoryQuery fields to actual API schema (query_text, not query).
      const apiBody: Record<string, unknown> = {
        collection: query.collection,
        query_text: query.query,
        top_k: query.top_k,
      };
      if (query.agent_filter) apiBody.agent_filter = query.agent_filter;
      if (query.type_filter) apiBody.type_filter = query.type_filter;

      const res = await fetch(`${SOULGRAPH_API}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(apiBody),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({ detail: res.statusText }));
        throw new Error(body.detail || `HTTP ${res.status}`);
      }
      const data: MemoryQueryResponse = await res.json();
      setResults(data.results ?? []);
      reportUsage('memory.search', {
        collection: query.collection,
        top_k: query.top_k,
        total_in_collection: data.total_in_collection,
        latency_ms: data.latency_ms,
      });
      return data.results ?? [];
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
      const res = await fetch(`${SOULGRAPH_API}/health`);
      if (!res.ok) {
        throw new Error(`Health check failed: HTTP ${res.status}`);
      }
      // Map actual API response (HealthResponse) to our MemoryHealthData shape.
      const raw = await res.json();
      const data: MemoryHealthData = {
        chromadb: raw.status === 'healthy' ? 'up' : 'down',
        collections: raw.collections ?? {},
        last_index: raw.timestamp ?? '',
      };
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
