import { useState, useEffect, useCallback, useRef } from 'react';
import { authFetch } from '../lib/api.ts';
import type { AgentStatus } from '../components/team/StatusDot.tsx';

/** Shape of a single team agent. */
export interface TeamAgent {
  name: string;
  role: string;
  status: AgentStatus;
  task: string;
  model: string;
  machine: string;
  unreadCount: number;
  contextPct: number;
  panePreview: string[];
}

// -- Agent role/model mappings (static metadata the backend doesn't track) ----

const AGENT_ROLES: Record<string, string> = {
  fury:    'CSO',
  shuri:   'CTO',
  happy:   'QA Lead',
  xavier:  'Coach',
  pepper:  'CPO',
  loki:    'CMO',
  hawkeye: 'Pipeline Ops',
  stark:   'Analyst',
  banner:  'Data Science',
};

const AGENT_MODELS: Record<string, string> = {
  fury:    'opus',
  shuri:   'opus',
  xavier:  'opus',
  happy:   'sonnet',
  pepper:  'sonnet',
  loki:    'sonnet',
  hawkeye: 'sonnet',
  stark:   'sonnet',
  banner:  'sonnet',
};

/** Designation-based sort order. Lower number = higher rank. Unknown agents sort last, alphabetically. */
export const AGENT_SORT_ORDER: Record<string, number> = {
  fury:    1,
  pepper:  2,
  shuri:   3,
  loki:    4,
  xavier:  5,
  hawkeye: 6,
  stark:   7,
  banner:  8,
  happy:   9,
};

/** Map backend AgentState to frontend TeamAgent shape. */
function mapAgent(raw: Record<string, unknown>): TeamAgent {
  const name = String(raw.name ?? '');
  const backendStatus = String(raw.status ?? 'offline');

  // Map backend status strings to AgentStatus type
  let status: AgentStatus = 'idle';
  if (backendStatus === 'working' || backendStatus === 'active') status = 'working';
  else if (backendStatus === 'blocked') status = 'blocked';
  else if (backendStatus === 'offline') status = 'offline';
  else if (backendStatus === 'idle') status = 'idle';

  // Parse pane preview lines if present (from ?preview=N query param).
  const rawPreview = raw.pane_preview;
  const panePreview: string[] = Array.isArray(rawPreview)
    ? (rawPreview as unknown[]).map((l) => String(l))
    : [];

  return {
    name,
    role: AGENT_ROLES[name] ?? 'Agent',
    status,
    task: String(raw.current_task ?? ''),
    model: AGENT_MODELS[name] ?? 'sonnet',
    machine: String(raw.machine ?? 'unknown'),
    unreadCount: Number(raw.inbox_count ?? 0),
    contextPct: 0, // Not tracked by backend yet
    panePreview,
  };
}

// -- Hook ---------------------------------------------------------------------

interface UseTeamAgentsOptions {
  /** Number of pane preview lines to fetch per agent (0 = none). */
  previewLines?: number;
}

interface UseTeamAgentsResult {
  agents: TeamAgent[];
  loading: boolean;
  error: string | null;
  refresh: () => void;
}

/**
 * useTeamAgents -- fetches agent state from GET /api/team/agents and polls every 5s.
 * Falls back to empty array if the team server is unreachable.
 */
export function useTeamAgents(options?: UseTeamAgentsOptions): UseTeamAgentsResult {
  const previewLines = options?.previewLines ?? 0;
  const [agents, setAgents] = useState<TeamAgent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchAgents = useCallback(async () => {
    try {
      const url = previewLines > 0
        ? `/api/team/agents?preview=${previewLines}`
        : '/api/team/agents';
      const res = await authFetch(url);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: Record<string, unknown>[] = await res.json();
      // Map and sort by designation hierarchy. Unknown agents sort last, alphabetically.
      const mapped = data.map(mapAgent).sort((a, b) => {
        const orderA = AGENT_SORT_ORDER[a.name] ?? 1000;
        const orderB = AGENT_SORT_ORDER[b.name] ?? 1000;
        if (orderA !== orderB) return orderA - orderB;
        return a.name.localeCompare(b.name);
      });
      setAgents(mapped);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch agents');
    } finally {
      setLoading(false);
    }
  }, [previewLines]);

  useEffect(() => {
    void fetchAgents();
    intervalRef.current = setInterval(() => { void fetchAgents(); }, 5000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [fetchAgents]);

  return { agents, loading, error, refresh: fetchAgents };
}
