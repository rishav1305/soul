import { useCallback } from 'react';
import { useTeamAgents } from '../../hooks/useTeamAgents.ts';
import { AgentCard } from './AgentCard.tsx';

interface TeamDashboardProps {
  onNavigate: (to: string) => void;
}

/**
 * TeamDashboard — CSS Grid of agent cards with 5s auto-refresh.
 * Route: /team (managed by TeamShell internal router)
 */
export default function TeamDashboard({ onNavigate }: TeamDashboardProps) {
  const { agents, loading, error, refresh } = useTeamAgents();

  const handleAgentClick = useCallback((name: string) => {
    onNavigate(`/team/${name}`);
  }, [onNavigate]);

  return (
    <div data-testid="team-dashboard" className="flex flex-col h-full bg-deep overflow-hidden">
      {/* ── Header ── */}
      <header className="flex items-center justify-between px-6 py-4 border-b border-border-subtle shrink-0">
        <div>
          <h1 className="text-base font-semibold text-fg">Soul Team</h1>
          <p className="text-[11px] text-fg-muted mt-0.5">
            {agents.filter((a) => a.status === 'working').length} working •{' '}
            {agents.filter((a) => a.status === 'idle').length} idle •{' '}
            {agents.filter((a) => a.status === 'blocked').length} blocked
          </p>
        </div>

        <div className="flex items-center gap-2">
          <span className="text-[10px] text-fg-muted">Refreshes every 5s</span>
          <button
            type="button"
            data-testid="team-dashboard-refresh"
            onClick={refresh}
            className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-overlay/60 transition-colors cursor-pointer"
            aria-label="Refresh agent status"
            title="Refresh"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M13.5 8A5.5 5.5 0 1 1 8 2.5c1.5 0 2.9.6 3.9 1.6L14 6" />
              <path d="M14 2v4h-4" />
            </svg>
          </button>
        </div>
      </header>

      {/* ── Content ── */}
      <div className="flex-1 overflow-y-auto p-6">
        {loading && agents.length === 0 && (
          <div data-testid="team-dashboard-loading" className="flex items-center justify-center h-48 text-sm text-fg-muted">
            Loading agents...
          </div>
        )}

        {error && (
          <div data-testid="team-dashboard-error" className="flex items-center justify-center h-48">
            <div className="text-center space-y-2">
              <p className="text-sm text-red-400">Failed to load agents</p>
              <p className="text-xs text-fg-muted">{error}</p>
              <button
                type="button"
                data-testid="team-dashboard-retry"
                onClick={refresh}
                className="px-3 py-1.5 text-xs bg-soul text-deep rounded hover:bg-soul/85 transition-colors cursor-pointer"
              >
                Retry
              </button>
            </div>
          </div>
        )}

        {agents.length > 0 && (
          <div
            data-testid="agent-grid"
            className="grid gap-4"
            style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))' }}
          >
            {agents.map((agent) => (
              <AgentCard
                key={agent.name}
                agent={agent}
                onClick={handleAgentClick}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
