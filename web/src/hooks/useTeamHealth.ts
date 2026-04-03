import { useState, useEffect, useCallback, useRef } from 'react';
import { authFetch } from '../lib/api.ts';

/** Health data types. */

export interface ServiceStatus {
  name: string;
  status: 'up' | 'down' | 'degraded';
  latencyMs?: number;
  lastChecked: number;
}

export interface AgentTokenUsage {
  agent: string;
  tokensUsed: number;
  tokensMax: number;
  costUsd: number;
}

export interface RestartEvent {
  id: string;
  agent: string;
  reason: string;
  timestamp: number;
}

export interface SystemResources {
  hostname: string;
  cpuTempC: number;
  memoryUsedMb: number;
  memoryTotalMb: number;
  loadAvg1: number;
  loadAvg5: number;
  loadAvg15: number;
}

export interface HealthAlert {
  id: string;
  level: 'info' | 'warn' | 'error';
  message: string;
  timestamp: number;
  agent?: string;
}

export interface TeamHealthData {
  services: ServiceStatus[];
  tokenUsage: AgentTokenUsage[];
  restartHistory: RestartEvent[];
  resources: SystemResources;
  alerts: HealthAlert[];
}

// -- Backend mapping ----------------------------------------------------------

/** Backend resource shape. */
interface BackendResources {
  hostname?: string;
  cpu_temp_c?: number;
  memory_used_mb?: number;
  memory_total_mb?: number;
  load_avg_1?: number;
  load_avg_5?: number;
  load_avg_15?: number;
}

/** Backend per-agent token shape. */
interface BackendTokenUsage {
  agent?: string;
  tokens_used?: number;
  cost_usd?: number;
}

/**
 * Map the backend HealthStatus response into the frontend TeamHealthData shape.
 * The backend now returns resources and token_usage; we map them to the
 * frontend types and fill defaults for fields not yet tracked.
 */
function mapHealthResponse(raw: Record<string, unknown>): TeamHealthData {
  const services: ServiceStatus[] = [];
  const svcMap = raw.services as Record<string, string> | undefined;
  if (svcMap && typeof svcMap === 'object') {
    for (const [name, status] of Object.entries(svcMap)) {
      services.push({
        name,
        status: status === 'ok' ? 'up' : status === 'degraded' ? 'degraded' : 'down',
        lastChecked: Date.now(),
      });
    }
  }

  // System resources from backend.
  const res = (raw.resources ?? {}) as BackendResources;

  // Token usage from backend (guardian.db 7-day aggregation).
  const rawTokens = (raw.token_usage ?? []) as BackendTokenUsage[];
  const tokenUsage: AgentTokenUsage[] = rawTokens.map((t) => ({
    agent: t.agent ?? '',
    tokensUsed: t.tokens_used ?? 0,
    tokensMax: 5_000_000, // Daily budget default (5M tokens)
    costUsd: t.cost_usd ?? 0,
  }));

  return {
    services,
    tokenUsage,
    restartHistory: [],    // Not yet tracked by backend health endpoint
    resources: {
      hostname: res.hostname ?? '',
      cpuTempC: res.cpu_temp_c ?? 0,
      memoryUsedMb: res.memory_used_mb ?? 0,
      memoryTotalMb: res.memory_total_mb ?? 0,
      loadAvg1: res.load_avg_1 ?? 0,
      loadAvg5: res.load_avg_5 ?? 0,
      loadAvg15: res.load_avg_15 ?? 0,
    },
    alerts: [],            // Not yet tracked by backend health endpoint
  };
}

// -- Hook ---------------------------------------------------------------------

interface UseTeamHealthResult {
  health: TeamHealthData | null;
  loading: boolean;
  error: string | null;
  refresh: () => void;
}

/**
 * useTeamHealth -- fetches team health data from GET /api/team/health, polling every 10s.
 */
export function useTeamHealth(): UseTeamHealthResult {
  const [health, setHealth] = useState<TeamHealthData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchHealth = useCallback(async () => {
    try {
      const res = await authFetch('/api/team/health');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: Record<string, unknown> = await res.json();
      setHealth(mapHealthResponse(data));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch health');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchHealth();
    intervalRef.current = setInterval(() => { void fetchHealth(); }, 10000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [fetchHealth]);

  return { health, loading, error, refresh: fetchHealth };
}
