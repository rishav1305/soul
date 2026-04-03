import { useState, useCallback } from 'react';
import { authFetch } from '../../lib/api.ts';
import { useTeamAgents } from '../../hooks/useTeamAgents.ts';
import { useAgentPane } from '../../hooks/useAgentPane.ts';
import { useTeamTasks } from '../../hooks/useTeamTasks.ts';
import { StatusDot } from './StatusDot.tsx';
import { ModelBadge } from './ModelBadge.tsx';
import { PaneViewer } from './PaneViewer.tsx';

interface AgentDetailProps {
  agentName: string;
  onNavigate: (to: string) => void;
}

/**
 * AgentDetail -- full-width pane viewer with compact info bar above.
 * Route: /team/:agent (managed by TeamShell internal router)
 */
export default function AgentDetail({ agentName, onNavigate }: AgentDetailProps) {
  const { agents } = useTeamAgents();
  const { lines, loading: paneLoading } = useAgentPane(agentName);
  const { tasks } = useTeamTasks();
  const [messageInput, setMessageInput] = useState('');
  const [sentMessages, setSentMessages] = useState<string[]>([]);
  const [showMessageInput, setShowMessageInput] = useState(false);
  const [sending, setSending] = useState(false);
  const [showBacklog, setShowBacklog] = useState(false);

  const agent = agents.find((a) => a.name === agentName);

  /** Tasks assigned to this agent, active stages first, capped at 5 for sidebar. */
  const agentTasks = tasks
    .filter((t) => t.assignee === agentName && t.stage !== 'done')
    .sort((a, b) => {
      const stagePriority: Record<string, number> = { 'in-progress': 0, blocked: 1, backlog: 2 };
      return (stagePriority[a.stage] ?? 3) - (stagePriority[b.stage] ?? 3);
    });

  const handleSend = useCallback(() => {
    const trimmed = messageInput.trim();
    if (!trimmed) return;

    setSending(true);

    authFetch('/api/team/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        to: agentName,
        body: trimmed,
        from: 'CEO',
        priority: 'P2',
      }),
    })
      .then(() => {
        setSentMessages((prev) => [...prev, trimmed]);
      })
      .catch(() => {
        setSentMessages((prev) => [...prev, trimmed]);
      })
      .finally(() => {
        setSending(false);
      });

    setMessageInput('');
    setShowMessageInput(false);
  }, [messageInput, agentName]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') handleSend();
    if (e.key === 'Escape') setShowMessageInput(false);
  }, [handleSend]);

  return (
    <div data-testid={`agent-detail-${agentName}`} className="flex flex-col h-full bg-deep overflow-hidden">
      {/* -- Header -- */}
      <header className="flex items-center gap-3 px-6 py-3 border-b border-border-subtle shrink-0">
        <button
          type="button"
          data-testid="agent-detail-back"
          onClick={() => onNavigate('/team')}
          className="flex items-center gap-1.5 text-fg-muted hover:text-fg transition-colors cursor-pointer text-sm"
          aria-label="Back to team dashboard"
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M10 3L5 8l5 5" />
          </svg>
          Team
        </button>

        <span className="text-fg-muted" aria-hidden="true">/</span>

        <div className="flex items-center gap-2 flex-1 min-w-0">
          {agent && <StatusDot status={agent.status} label={`${agentName} is ${agent.status}`} />}
          <h1 className="text-sm font-semibold text-fg">{agentName}</h1>
          {agent && <ModelBadge model={agent.model} />}
          {agent && (
            <span className="text-[10px] text-fg-muted hidden sm:block">{agent.role}</span>
          )}
        </div>

        <button
          type="button"
          data-testid="agent-detail-send-message-btn"
          onClick={() => setShowMessageInput(true)}
          className="px-3 py-1.5 text-xs bg-soul text-deep rounded hover:bg-soul/85 transition-colors cursor-pointer font-medium"
        >
          Send Message
        </button>
      </header>

      {/* -- Inline message input -- */}
      {showMessageInput && (
        <div className="flex items-center gap-2 px-6 py-2 border-b border-border-subtle bg-surface/60 shrink-0">
          <input
            data-testid="agent-message-input"
            type="text"
            value={messageInput}
            onChange={(e) => setMessageInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={`Message to ${agentName}...`}
            className="flex-1 bg-deep border border-border-default rounded px-3 py-1.5 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul/50"
            autoFocus
          />
          <button
            type="button"
            data-testid="agent-message-send-btn"
            onClick={handleSend}
            disabled={!messageInput.trim() || sending}
            className="px-3 py-1.5 text-xs bg-soul text-deep rounded hover:bg-soul/85 disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer font-medium"
          >
            Send
          </button>
          <button
            type="button"
            data-testid="agent-message-cancel-btn"
            onClick={() => setShowMessageInput(false)}
            className="px-3 py-1.5 text-xs text-fg-muted hover:text-fg transition-colors cursor-pointer"
          >
            Cancel
          </button>
        </div>
      )}

      {/* -- Compact info bar (replaces sidebar) -- */}
      <div className="flex items-center gap-4 px-6 py-2 border-b border-border-subtle bg-surface/30 shrink-0 text-xs overflow-x-auto">
        {/* Current task */}
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <span className="text-fg-muted font-medium uppercase text-[10px] tracking-wider shrink-0">Task</span>
          {agent?.task ? (
            <span data-testid="agent-info-task" className="text-fg-secondary truncate">{agent.task}</span>
          ) : (
            <span className="text-fg-muted italic">None</span>
          )}
        </div>

        {/* Context usage */}
        <div className="flex items-center gap-2 shrink-0">
          <span className="text-fg-muted font-medium uppercase text-[10px] tracking-wider">Context</span>
          <div
            className="w-16 h-1.5 bg-overlay rounded-full overflow-hidden"
            role="progressbar"
            aria-valuenow={agent?.contextPct ?? 0}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`Context usage: ${agent?.contextPct ?? 0}%`}
          >
            <div
              data-testid="agent-context-bar"
              className={`h-full rounded-full transition-all ${
                (agent?.contextPct ?? 0) >= 80 ? 'bg-red-500' :
                (agent?.contextPct ?? 0) >= 60 ? 'bg-yellow-500' :
                'bg-green-500'
              }`}
              style={{ width: `${agent?.contextPct ?? 0}%` }}
            />
          </div>
          <span className="text-fg-muted text-[10px]">{agent?.contextPct ?? 0}%</span>
        </div>

        {/* Machine */}
        <div className="flex items-center gap-2 shrink-0">
          <span className="text-fg-muted font-medium uppercase text-[10px] tracking-wider">Machine</span>
          <span className="text-fg-secondary">{agent?.machine ?? 'unknown'}</span>
        </div>

        {/* Backlog toggle */}
        {agentTasks.length > 0 && (
          <button
            type="button"
            data-testid="agent-backlog-toggle"
            onClick={() => setShowBacklog(!showBacklog)}
            className="flex items-center gap-1 text-fg-muted hover:text-fg transition-colors cursor-pointer shrink-0"
          >
            <span className="font-medium uppercase text-[10px] tracking-wider">Backlog</span>
            <span className="text-soul text-[10px] font-bold">{agentTasks.length}</span>
            <svg
              width="10"
              height="10"
              viewBox="0 0 16 16"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              className={`transition-transform ${showBacklog ? 'rotate-180' : ''}`}
            >
              <path d="M4 6l4 4 4-4" />
            </svg>
          </button>
        )}

        {/* Sent messages count */}
        {sentMessages.length > 0 && (
          <span className="text-[10px] text-fg-muted shrink-0">
            {sentMessages.length} sent
          </span>
        )}
      </div>

      {/* -- Collapsible backlog panel -- */}
      {showBacklog && (
        <div
          data-testid="agent-backlog"
          className="px-6 py-2 border-b border-border-subtle bg-surface/20 shrink-0 max-h-40 overflow-y-auto"
          aria-label={`Task backlog for ${agentName}`}
        >
          <ul className="space-y-1" aria-label="Agent tasks">
            {agentTasks.map((task) => (
              <li
                key={task.id}
                data-testid={`agent-task-${task.id}`}
                className="flex items-center gap-2 rounded px-2 py-1 bg-surface/40 hover:bg-surface/70 transition-colors text-xs"
              >
                <span
                  aria-label={`Priority: ${task.priority}`}
                  className={`w-1.5 h-1.5 rounded-full shrink-0 ${
                    task.priority === 'critical' ? 'bg-red-500' :
                    task.priority === 'high'     ? 'bg-orange-400' :
                    task.priority === 'medium'   ? 'bg-yellow-400' :
                    'bg-zinc-500'
                  }`}
                />
                <span className="text-fg flex-1 min-w-0 truncate">{task.title}</span>
                <span
                  className={`text-[10px] font-medium shrink-0 ${
                    task.stage === 'in-progress' ? 'text-green-400' :
                    task.stage === 'blocked'     ? 'text-yellow-400' :
                    'text-fg-muted'
                  }`}
                >
                  {task.stage === 'in-progress' ? 'active' : task.stage}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* -- Full-width pane viewer (takes remaining space) -- */}
      <div className="flex-1 min-h-0 flex flex-col">
        <div className="px-4 py-2 border-b border-border-subtle shrink-0 flex items-center gap-2">
          <span className="text-[11px] text-fg-muted font-medium uppercase tracking-wider">Live Output</span>
          {paneLoading && (
            <span className="text-[10px] text-fg-muted italic">Loading...</span>
          )}
          <span className="text-[10px] text-fg-muted ml-auto">{lines.length} lines</span>
        </div>
        <PaneViewer
          lines={lines}
          autoScroll={true}
          className="flex-1"
        />
      </div>
    </div>
  );
}
