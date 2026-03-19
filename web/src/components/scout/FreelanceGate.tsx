import { useState } from 'react';
import type { ScoutLead } from '../../hooks/useScout';
import { api } from '../../lib/api';

interface FreelanceGateProps {
  lead: ScoutLead;
  onAction: (action: string, leadId: number) => void;
}

interface ScoreBreakdown {
  skill_match: number;
  budget_fit: number;
  scope_clarity: number;
  client_quality: number;
  time_fit: number;
}

interface ScoreResponse {
  result: string;
  breakdown?: ScoreBreakdown;
}

interface SectionState<T> {
  content: T | null;
  loading: boolean;
  error: string | null;
}

const SCORE_FIELDS: { key: keyof ScoreBreakdown; label: string }[] = [
  { key: 'skill_match', label: 'Skill Match' },
  { key: 'budget_fit', label: 'Budget Fit' },
  { key: 'scope_clarity', label: 'Scope Clarity' },
  { key: 'client_quality', label: 'Client Quality' },
  { key: 'time_fit', label: 'Time Fit' },
];

function scoreBarColor(value: number): string {
  if (value > 70) return 'bg-emerald-400';
  if (value > 40) return 'bg-amber-400';
  return 'bg-red-400';
}

function scoreBadgeColor(score: number): string {
  if (score >= 80) return 'bg-emerald-500/20 text-emerald-400';
  if (score >= 50) return 'bg-amber-500/20 text-amber-400';
  return 'bg-zinc-500/20 text-zinc-400';
}

function parseBreakdown(result: string): ScoreBreakdown | null {
  try {
    const parsed: unknown = JSON.parse(result);
    if (
      parsed !== null &&
      typeof parsed === 'object' &&
      'skill_match' in (parsed as Record<string, unknown>)
    ) {
      return parsed as ScoreBreakdown;
    }
  } catch {
    // Not JSON — ignore
  }
  return null;
}

export function FreelanceGate({ lead, onAction }: FreelanceGateProps) {
  const [score, setScore] = useState<SectionState<ScoreBreakdown>>({
    content: null,
    loading: false,
    error: null,
  });
  const [proposal, setProposal] = useState<SectionState<string>>({
    content: null,
    loading: false,
    error: null,
  });

  const handleGenerateScore = async () => {
    setScore({ content: null, loading: true, error: null });
    try {
      const result = await api.post<ScoreResponse>('/api/ai/freelance-score', { lead_id: lead.id });
      const breakdown = result?.breakdown ?? parseBreakdown(result?.result ?? '');
      if (breakdown) {
        setScore({ content: breakdown, loading: false, error: null });
      } else {
        setScore({ content: null, loading: false, error: 'Could not parse score breakdown' });
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setScore({ content: null, loading: false, error: message });
    }
  };

  const handleGenerateProposal = async () => {
    setProposal({ content: null, loading: true, error: null });
    try {
      const result = await api.post<{ result: string }>('/api/ai/proposal', {
        lead_id: lead.id,
        platform: 'upwork',
      });
      setProposal({ content: result?.result ?? '', loading: false, error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setProposal({ content: null, loading: false, error: message });
    }
  };

  return (
    <div className="bg-surface rounded-lg p-4 space-y-4" data-testid="freelance-gate">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-lg font-semibold text-fg truncate">{lead.title}</h3>
          <p className="text-sm text-fg-muted">{lead.company}</p>
        </div>
        <span
          className={`shrink-0 px-2 py-0.5 text-xs rounded-full font-medium ${scoreBadgeColor(lead.match_score)}`}
          data-testid="freelance-gate-score-badge"
        >
          {lead.match_score}%
        </span>
      </div>

      {/* Gig Score Section */}
      <div className="bg-elevated rounded-lg p-3 space-y-2" data-testid="freelance-gate-section-score">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-medium text-fg">Gig Score</h4>
          {!score.content && !score.loading && (
            <button
              onClick={handleGenerateScore}
              className="px-2.5 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
              data-testid="freelance-gate-generate-score"
            >
              Generate
            </button>
          )}
        </div>
        {score.loading && (
          <div className="flex items-center gap-2 text-sm text-fg-muted">
            <div className="w-3 h-3 border-2 border-soul/30 border-t-soul rounded-full animate-spin" />
            Analyzing gig...
          </div>
        )}
        {score.error && (
          <div className="text-sm text-red-400 bg-red-500/10 rounded px-3 py-2" data-testid="freelance-gate-error-score">
            {score.error}
          </div>
        )}
        {score.content && (
          <div className="space-y-2" data-testid="freelance-gate-content-score">
            {SCORE_FIELDS.map(({ key, label }) => {
              const value = score.content ? score.content[key] : 0;
              return (
                <div key={key} className="space-y-0.5">
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-fg-muted">{label}</span>
                    <span className="text-fg font-medium">{value}</span>
                  </div>
                  <div className="h-1.5 bg-deep rounded-full overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all ${scoreBarColor(value)}`}
                      style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Proposal Draft Section */}
      <div className="bg-elevated rounded-lg p-3 space-y-2" data-testid="freelance-gate-section-proposal">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-medium text-fg">Proposal Draft</h4>
          {!proposal.content && !proposal.loading && (
            <button
              onClick={handleGenerateProposal}
              className="px-2.5 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
              data-testid="freelance-gate-generate-proposal"
            >
              Generate
            </button>
          )}
        </div>
        {proposal.loading && (
          <div className="flex items-center gap-2 text-sm text-fg-muted">
            <div className="w-3 h-3 border-2 border-soul/30 border-t-soul rounded-full animate-spin" />
            Drafting proposal...
          </div>
        )}
        {proposal.error && (
          <div className="text-sm text-red-400 bg-red-500/10 rounded px-3 py-2" data-testid="freelance-gate-error-proposal">
            {proposal.error}
          </div>
        )}
        {proposal.content && (
          <pre
            className="text-sm text-fg whitespace-pre-wrap bg-deep rounded px-3 py-2 overflow-auto max-h-64"
            data-testid="freelance-gate-content-proposal"
          >
            {proposal.content}
          </pre>
        )}
      </div>

      {/* Footer Actions */}
      <div className="flex items-center justify-end gap-2 pt-2 border-t border-border-subtle">
        <button
          onClick={() => onAction('skip', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-zinc-500/20 text-zinc-400 hover:bg-zinc-500/30 transition-colors"
          data-testid="freelance-gate-skip"
        >
          Skip
        </button>
        <button
          onClick={() => onAction('edit', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 transition-colors"
          data-testid="freelance-gate-edit"
        >
          Edit
        </button>
        <button
          onClick={() => onAction('submit', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors"
          data-testid="freelance-gate-submit"
        >
          Submit Proposal
        </button>
      </div>
    </div>
  );
}
