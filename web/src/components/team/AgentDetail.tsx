import { useState, useCallback } from 'react';
import { useTeamAgents } from '../../hooks/useTeamAgents.ts';
import { useAgentPane } from '../../hooks/useAgentPane.ts';
import { StatusDot } from './StatusDot.tsx';
import { ModelBadge } from './ModelBadge.tsx';
import { PaneViewer } from './PaneViewer.tsx';

interface AgentDetailProps {
  agentName: string;
  onNavigate: (to: string) => void;
}

/**
 * AgentDetail — split panel: left pane output, right sidebar with tasks/messages/memory.
 * Route: /team/:agent (managed by TeamShell internal router)
 */
export default function AgentDetail({ agentName, onNavigate }: AgentDetailProps) {
  const { agents } = useTeamAgents();
  const { lines, loading: paneLoading } = useAgentPane(agentName);
  const [messageInput, setMessageInput] = useState('');
  const [sentMessages, setSentMessages] = useState<string[]>([]);
  const [showMessageInput, setShowMessageInput] = useState(false);
  const [sending, setSending] = useState(false);

  const agent = agents.find((a) => a.name === agentName);

  const handleSend = useCallback(() => {
    const trimmed = messageInput.trim();
    if (!trimmed) return;

    setSending(true);

    // Send via the real API (POST /api/team/chat with { to, body, from, priority })
    fetch('/api/team/chat', {
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
        // Optimistic: still show locally on failure
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
      {/* ── Header ── */}
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

      {/* ── Inline message input ── */}
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

      {/* ── Main split layout ── */}
      <div className="flex flex-1 min-h-0 overflow-hidden">
        {/* Left: Pane viewer */}
        <div className="flex-1 min-w-0 flex flex-col border-r border-border-subtle">
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

        {/* Right: Sidebar */}
        <aside
          data-testid="agent-detail-sidebar"
          className="w-72 shrink-0 flex flex-col overflow-hidden"
          aria-label="Agent details sidebar"
        >
          {/* Current task */}
          <section className="px-4 py-3 border-b border-border-subtle">
            <h2 className="text-[11px] text-fg-muted font-medium uppercase tracking-wider mb-2">Current Task</h2>
            {agent?.task ? (
              <p className="text-sm text-fg-secondary leading-relaxed">{agent.task}</p>
            ) : (
              <p className="text-sm text-fg-muted italic">No active task</p>
            )}
          </section>

          {/* Memory usage */}
          <section className="px-4 py-3 border-b border-border-subtle">
            <div className="flex items-center justify-between mb-2">
              <h2 className="text-[11px] text-fg-muted font-medium uppercase tracking-wider">Context</h2>
              <span className="text-[10px] text-fg-muted">{agent?.contextPct ?? 0}%</span>
            </div>
            <div
              className="w-full h-1.5 bg-overlay rounded-full overflow-hidden"
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
          </section>

          {/* Backlog placeholder */}
          <section className="px-4 py-3 border-b border-border-subtle flex-1 overflow-y-auto">
            <h2 className="text-[11px] text-fg-muted font-medium uppercase tracking-wider mb-2">Backlog</h2>
            <p className="text-xs text-fg-muted italic">Wiring up task backlog API...</p>
          </section>

          {/* Sent messages */}
          {sentMessages.length > 0 && (
            <section className="px-4 py-3 border-t border-border-subtle">
              <h2 className="text-[11px] text-fg-muted font-medium uppercase tracking-wider mb-2">Sent</h2>
              <div className="space-y-1">
                {sentMessages.map((m, i) => (
                  <div key={i} className="text-xs text-fg-secondary bg-surface/60 rounded px-2 py-1 line-clamp-2">
                    {m}
                  </div>
                ))}
              </div>
            </section>
          )}

          {/* Machine */}
          <section className="px-4 py-3 border-t border-border-subtle shrink-0">
            <div className="flex items-center justify-between text-[10px] text-fg-muted">
              <span>Machine</span>
              <span className="text-fg-secondary">{agent?.machine ?? 'unknown'}</span>
            </div>
          </section>
        </aside>
      </div>
    </div>
  );
}
