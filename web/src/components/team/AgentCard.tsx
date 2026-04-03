import type { TeamAgent } from '../../hooks/useTeamAgents.ts';
import { StatusDot } from './StatusDot.tsx';
import { ModelBadge } from './ModelBadge.tsx';

/** AgentCard — reusable card for the team dashboard grid. */

interface AgentCardProps {
  agent: TeamAgent;
  onClick: (name: string) => void;
}

const MACHINE_COLOR: Record<string, string> = {
  'titan-pi': 'text-purple-400',
  'titan-pc': 'text-blue-400',
};

export function AgentCard({ agent, onClick }: AgentCardProps) {
  return (
    <button
      type="button"
      data-testid={`agent-card-${agent.name}`}
      onClick={() => onClick(agent.name)}
      className="w-full text-left bg-surface hover:bg-elevated border border-border-subtle hover:border-border-default rounded-xl p-4 transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-soul/50 cursor-pointer"
      aria-label={`${agent.name} — ${agent.role}, status: ${agent.status}`}
    >
      {/* Header row */}
      <div className="flex items-start justify-between gap-2 mb-3">
        <div className="flex items-center gap-2 min-w-0">
          <StatusDot status={agent.status} size="md" label={`${agent.name} is ${agent.status}`} />
          <span className="font-semibold text-sm text-fg truncate">{agent.name}</span>
        </div>
        <ModelBadge model={agent.model} />
      </div>

      {/* Role */}
      <div className="text-[11px] text-fg-muted mb-2 uppercase tracking-wider">{agent.role}</div>

      {/* Current task */}
      {agent.task ? (
        <p className="text-xs text-fg-secondary line-clamp-2 leading-relaxed">{agent.task}</p>
      ) : (
        <p className="text-xs text-fg-muted italic">No active task</p>
      )}

      {/* Machine badge */}
      <div className="mt-3 pt-3 border-t border-border-subtle flex items-center justify-between">
        <span className={`text-[10px] font-medium ${MACHINE_COLOR[agent.machine] ?? 'text-fg-muted'}`}>
          {agent.machine}
        </span>
        {agent.unreadCount > 0 && (
          <span
            data-testid={`agent-unread-${agent.name}`}
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
