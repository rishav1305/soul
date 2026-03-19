import { useState } from 'react';
import type { ScoutLead } from '../../hooks/useScout';
import { api } from '../../lib/api';

interface NetworkingGateProps {
  lead: ScoutLead;
  onAction: (action: string, leadId: number) => void;
}

type Channel = 'linkedin' | 'x' | 'email';

interface DraftState {
  content: string;
  loading: boolean;
  error: string | null;
}

const CHANNELS: { key: Channel; label: string }[] = [
  { key: 'linkedin', label: 'LinkedIn' },
  { key: 'x', label: 'X' },
  { key: 'email', label: 'Email' },
];

function warmthBadge(score: number): { label: string; className: string } {
  if (score >= 70) return { label: 'Warm', className: 'bg-emerald-500/20 text-emerald-400' };
  if (score >= 40) return { label: 'Lukewarm', className: 'bg-amber-500/20 text-amber-400' };
  return { label: 'Cold', className: 'bg-blue-500/20 text-blue-400' };
}

export function NetworkingGate({ lead, onAction }: NetworkingGateProps) {
  const [activeChannel, setActiveChannel] = useState<Channel>('linkedin');
  const [drafts, setDrafts] = useState<Record<Channel, DraftState>>({
    linkedin: { content: '', loading: false, error: null },
    x: { content: '', loading: false, error: null },
    email: { content: '', loading: false, error: null },
  });

  const handleGenerate = async (channel: Channel) => {
    setDrafts(prev => ({
      ...prev,
      [channel]: { content: '', loading: true, error: null },
    }));
    try {
      const result = await api.post<{ result: string }>('/api/ai/networking-draft', {
        lead_id: lead.id,
        channel,
      });
      setDrafts(prev => ({
        ...prev,
        [channel]: { content: result?.result ?? '', loading: false, error: null },
      }));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setDrafts(prev => ({
        ...prev,
        [channel]: { content: '', loading: false, error: message },
      }));
    }
  };

  const warmth = warmthBadge(lead.match_score);
  const contactName = lead.contact || lead.title;
  const currentDraft = drafts[activeChannel];

  return (
    <div className="bg-surface rounded-lg p-4 space-y-4" data-testid="networking-gate">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-lg font-semibold text-fg truncate">{contactName}</h3>
          <p className="text-sm text-fg-muted">{lead.company}</p>
        </div>
        <span
          className={`shrink-0 px-2 py-0.5 text-xs rounded-full font-medium ${warmth.className}`}
          data-testid="networking-gate-warmth"
        >
          {warmth.label}
        </span>
      </div>

      {/* Channel Tabs */}
      <div className="flex gap-1 bg-elevated rounded-lg p-1" data-testid="networking-gate-channels">
        {CHANNELS.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setActiveChannel(key)}
            className={`flex-1 px-3 py-1.5 text-sm rounded-md transition-colors ${
              activeChannel === key
                ? 'bg-soul/20 text-soul'
                : 'bg-transparent text-fg-muted hover:text-fg-secondary'
            }`}
            data-testid={`networking-gate-channel-${key}`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Draft Display Area */}
      <div className="bg-elevated rounded-lg p-3 space-y-2" data-testid={`networking-gate-section-${activeChannel}`}>
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-medium text-fg">
            {CHANNELS.find(c => c.key === activeChannel)?.label} Draft
          </h4>
          {!currentDraft.content && !currentDraft.loading && (
            <button
              onClick={() => handleGenerate(activeChannel)}
              className="px-2.5 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
              data-testid={`networking-gate-generate-${activeChannel}`}
            >
              Generate
            </button>
          )}
        </div>
        {currentDraft.loading && (
          <div className="flex items-center gap-2 text-sm text-fg-muted">
            <div className="w-3 h-3 border-2 border-soul/30 border-t-soul rounded-full animate-spin" />
            Drafting message...
          </div>
        )}
        {currentDraft.error && (
          <div
            className="text-sm text-red-400 bg-red-500/10 rounded px-3 py-2"
            data-testid={`networking-gate-error-${activeChannel}`}
          >
            {currentDraft.error}
          </div>
        )}
        {currentDraft.content && (
          <pre
            className="text-sm text-fg whitespace-pre-wrap bg-deep rounded px-3 py-2 overflow-auto max-h-64"
            data-testid={`networking-gate-content-${activeChannel}`}
          >
            {currentDraft.content}
          </pre>
        )}
      </div>

      {/* Footer Actions */}
      <div className="flex items-center justify-end gap-2 pt-2 border-t border-border-subtle">
        <button
          onClick={() => onAction('skip', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-zinc-500/20 text-zinc-400 hover:bg-zinc-500/30 transition-colors"
          data-testid="networking-gate-skip"
        >
          Skip
        </button>
        <button
          onClick={() => onAction('edit', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 transition-colors"
          data-testid="networking-gate-edit"
        >
          Edit
        </button>
        <button
          onClick={() => onAction('send', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors"
          data-testid="networking-gate-send"
        >
          Send
        </button>
      </div>
    </div>
  );
}
