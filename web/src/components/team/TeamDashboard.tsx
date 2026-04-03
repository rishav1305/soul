import { useCallback, useState, useEffect } from 'react';
import { useTeamAgents } from '../../hooks/useTeamAgents.ts';
import type { TeamAgent } from '../../hooks/useTeamAgents.ts';
import { AgentCard } from './AgentCard.tsx';
import { StatusDot } from './StatusDot.tsx';
import { ModelBadge } from './ModelBadge.tsx';

type ViewMode = 'list' | 'tiles' | 'big';

const VIEW_MODE_KEY = 'soul-team-view-mode';

interface TeamDashboardProps {
  onNavigate: (to: string) => void;
}

const MACHINE_COLOR: Record<string, string> = {
  'titan-pi': 'text-purple-400',
  'titan-pc': 'text-blue-400',
};

/** Compact list row for an agent. */
function AgentListRow({ agent, onClick }: { agent: TeamAgent; onClick: (name: string) => void }) {
  return (
    <button
      type="button"
      data-testid={`agent-list-${agent.name}`}
      onClick={() => onClick(agent.name)}
      className="w-full flex items-center gap-3 px-4 py-2.5 text-left hover:bg-surface/70 transition-colors cursor-pointer border-b border-border-subtle last:border-b-0"
      aria-label={`${agent.name} -- ${agent.role}, status: ${agent.status}`}
    >
      <StatusDot status={agent.status} size="sm" label={`${agent.name} is ${agent.status}`} />
      <span className="font-semibold text-sm text-fg w-20 shrink-0">{agent.name}</span>
      <span className="text-[11px] text-fg-muted uppercase tracking-wider w-24 shrink-0">{agent.role}</span>
      <ModelBadge model={agent.model} />
      <span className="text-xs text-fg-secondary flex-1 min-w-0 truncate">
        {agent.task || <span className="text-fg-muted italic">No active task</span>}
      </span>
      <span className={`text-[10px] font-medium w-16 text-right shrink-0 ${MACHINE_COLOR[agent.machine] ?? 'text-fg-muted'}`}>
        {agent.machine}
      </span>
      <span className="text-[10px] text-fg-muted w-14 text-right shrink-0">
        {agent.unreadCount > 0 ? `${agent.unreadCount} msgs` : '--'}
      </span>
    </button>
  );
}

/** Big tile card with pane preview snippet. */
function AgentBigCard({ agent, onClick }: { agent: TeamAgent; onClick: (name: string) => void }) {
  return (
    <button
      type="button"
      data-testid={`agent-big-${agent.name}`}
      onClick={() => onClick(agent.name)}
      className="w-full text-left bg-surface hover:bg-elevated border border-border-subtle hover:border-border-default rounded-xl p-5 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-soul/50 cursor-pointer"
      aria-label={`${agent.name} -- ${agent.role}, status: ${agent.status}`}
    >
      {/* Header row */}
      <div className="flex items-start justify-between gap-2 mb-2">
        <div className="flex items-center gap-2 min-w-0">
          <StatusDot status={agent.status} size="md" label={`${agent.name} is ${agent.status}`} />
          <span className="font-semibold text-base text-fg truncate">{agent.name}</span>
        </div>
        <ModelBadge model={agent.model} />
      </div>

      {/* Role */}
      <div className="text-[11px] text-fg-muted mb-2 uppercase tracking-wider">{agent.role}</div>

      {/* Current task */}
      {agent.task ? (
        <p className="text-xs text-fg-secondary line-clamp-2 leading-relaxed mb-3">{agent.task}</p>
      ) : (
        <p className="text-xs text-fg-muted italic mb-3">No active task</p>
      )}

      {/* Pane preview snippet */}
      {agent.panePreview.length > 0 && (
        <div
          data-testid={`agent-preview-${agent.name}`}
          className="rounded bg-deep border border-border-subtle p-2 mb-3 font-mono text-[10px] leading-snug text-green-400 overflow-hidden"
          style={{ fontFamily: "'JetBrains Mono', 'Fira Code', ui-monospace, monospace" }}
        >
          {agent.panePreview.map((line, i) => (
            <div key={`${i}-${line.slice(0, 8)}`} className="truncate">
              {line || '\u00A0'}
            </div>
          ))}
        </div>
      )}

      {/* Footer: machine, messages, last active */}
      <div className="pt-3 border-t border-border-subtle flex items-center justify-between gap-2 text-[10px]">
        <span className={`font-medium ${MACHINE_COLOR[agent.machine] ?? 'text-fg-muted'}`}>
          {agent.machine}
        </span>
        <span className="text-fg-muted">
          {agent.unreadCount > 0 ? `${agent.unreadCount} msgs` : '--'}
        </span>
        {agent.unreadCount > 0 && (
          <span
            data-testid={`agent-unread-big-${agent.name}`}
            className="inline-flex items-center justify-center w-4 h-4 text-[9px] font-bold rounded-full bg-soul text-deep"
            aria-label={`${agent.unreadCount} unread messages`}
          >
            {agent.unreadCount > 9 ? '9+' : agent.unreadCount}
          </span>
        )}
      </div>
    </button>
  );
}

/**
 * TeamDashboard -- CSS Grid of agent cards with 5s auto-refresh and 3 view modes.
 * Route: /team (managed by TeamShell internal router)
 */
export default function TeamDashboard({ onNavigate }: TeamDashboardProps) {
  const [viewMode, setViewMode] = useState<ViewMode>(() => {
    const stored = localStorage.getItem(VIEW_MODE_KEY);
    if (stored === 'list' || stored === 'tiles' || stored === 'big') return stored;
    return 'tiles';
  });

  // Fetch preview lines only in big tile mode.
  const { agents, loading, error, refresh } = useTeamAgents({
    previewLines: viewMode === 'big' ? 3 : 0,
  });

  // Persist view mode to localStorage.
  useEffect(() => {
    localStorage.setItem(VIEW_MODE_KEY, viewMode);
  }, [viewMode]);

  const handleAgentClick = useCallback((name: string) => {
    onNavigate(`/team/${name}`);
  }, [onNavigate]);

  return (
    <div data-testid="team-dashboard" className="flex flex-col h-full bg-deep overflow-hidden">
      {/* -- Header -- */}
      <header className="flex items-center justify-between px-6 py-4 border-b border-border-subtle shrink-0">
        <div>
          <h1 className="text-base font-semibold text-fg">Soul Team</h1>
          <p className="text-[11px] text-fg-muted mt-0.5">
            {agents.filter((a) => a.status === 'working').length} working{' '}
            {agents.filter((a) => a.status === 'idle').length} idle{' '}
            {agents.filter((a) => a.status === 'blocked').length} blocked
          </p>
        </div>

        <div className="flex items-center gap-3">
          {/* View mode toggle */}
          <div
            data-testid="view-mode-toggle"
            className="flex items-center bg-surface rounded-lg border border-border-subtle overflow-hidden"
            role="radiogroup"
            aria-label="View mode"
          >
            <button
              type="button"
              data-testid="view-mode-list"
              onClick={() => setViewMode('list')}
              className={`px-2.5 py-1.5 text-[10px] font-medium transition-colors cursor-pointer ${
                viewMode === 'list'
                  ? 'bg-soul text-deep'
                  : 'text-fg-muted hover:text-fg'
              }`}
              role="radio"
              aria-checked={viewMode === 'list'}
              aria-label="List view"
              title="List view"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                <path d="M2 4h12M2 8h12M2 12h12" />
              </svg>
            </button>
            <button
              type="button"
              data-testid="view-mode-tiles"
              onClick={() => setViewMode('tiles')}
              className={`px-2.5 py-1.5 text-[10px] font-medium transition-colors cursor-pointer ${
                viewMode === 'tiles'
                  ? 'bg-soul text-deep'
                  : 'text-fg-muted hover:text-fg'
              }`}
              role="radio"
              aria-checked={viewMode === 'tiles'}
              aria-label="Tile view"
              title="Tile view"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <rect x="1" y="1" width="6" height="6" rx="1" />
                <rect x="9" y="1" width="6" height="6" rx="1" />
                <rect x="1" y="9" width="6" height="6" rx="1" />
                <rect x="9" y="9" width="6" height="6" rx="1" />
              </svg>
            </button>
            <button
              type="button"
              data-testid="view-mode-big"
              onClick={() => setViewMode('big')}
              className={`px-2.5 py-1.5 text-[10px] font-medium transition-colors cursor-pointer ${
                viewMode === 'big'
                  ? 'bg-soul text-deep'
                  : 'text-fg-muted hover:text-fg'
              }`}
              role="radio"
              aria-checked={viewMode === 'big'}
              aria-label="Big tile view"
              title="Big tile view"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <rect x="1" y="1" width="14" height="6" rx="1" />
                <rect x="1" y="9" width="14" height="6" rx="1" />
              </svg>
            </button>
          </div>

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

      {/* -- Content -- */}
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

        {agents.length > 0 && viewMode === 'list' && (
          <div
            data-testid="agent-list"
            className="bg-surface border border-border-subtle rounded-xl overflow-hidden"
          >
            {agents.map((agent) => (
              <AgentListRow key={agent.name} agent={agent} onClick={handleAgentClick} />
            ))}
          </div>
        )}

        {agents.length > 0 && viewMode === 'tiles' && (
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

        {agents.length > 0 && viewMode === 'big' && (
          <div
            data-testid="agent-grid-big"
            className="grid gap-4"
            style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))' }}
          >
            {agents.map((agent) => (
              <AgentBigCard key={agent.name} agent={agent} onClick={handleAgentClick} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
