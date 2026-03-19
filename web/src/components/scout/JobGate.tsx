import { useState } from 'react';
import type { ScoutLead } from '../../hooks/useScout';
import { api } from '../../lib/api';

interface JobGateProps {
  lead: ScoutLead;
  onAction: (action: string, leadId: number) => void;
}

type SectionName = 'resume' | 'cover' | 'outreach';

interface SectionState {
  content: string;
  loading: boolean;
  error: string | null;
}

const SECTION_CONFIG: { name: SectionName; label: string; endpoint: string; bodyKey?: string }[] = [
  { name: 'resume', label: 'Resume Diff', endpoint: '/api/ai/resume-tailor' },
  { name: 'cover', label: 'Cover Letter', endpoint: '/api/ai/cover-letter' },
  { name: 'outreach', label: 'Outreach Draft', endpoint: '/api/ai/outreach' },
];

function tierBadge(score: number): { label: string; className: string } {
  if (score >= 80) return { label: 'Tier 1', className: 'bg-emerald-500/20 text-emerald-400' };
  if (score >= 60) return { label: 'Tier 2', className: 'bg-amber-500/20 text-amber-400' };
  return { label: 'Tier 3', className: 'bg-zinc-500/20 text-zinc-400' };
}

export function JobGate({ lead, onAction }: JobGateProps) {
  const [sections, setSections] = useState<Record<SectionName, SectionState>>({
    resume: { content: '', loading: false, error: null },
    cover: { content: '', loading: false, error: null },
    outreach: { content: '', loading: false, error: null },
  });

  const handleGenerate = async (name: SectionName, endpoint: string) => {
    setSections(prev => ({
      ...prev,
      [name]: { content: '', loading: true, error: null },
    }));
    try {
      const result = await api.post<{ result: string }>(endpoint, { lead_id: lead.id });
      setSections(prev => ({
        ...prev,
        [name]: { content: result?.result ?? '', loading: false, error: null },
      }));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setSections(prev => ({
        ...prev,
        [name]: { content: '', loading: false, error: message },
      }));
    }
  };

  const tier = tierBadge(lead.match_score);

  return (
    <div className="bg-surface rounded-lg p-4 space-y-4" data-testid="job-gate">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-lg font-semibold text-fg truncate">{lead.title}</h3>
          <p className="text-sm text-fg-muted">{lead.company}</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <span
            className={`px-2 py-0.5 text-xs rounded-full font-medium ${tier.className}`}
            data-testid="job-gate-tier"
          >
            {tier.label}
          </span>
          <span
            className={`px-2 py-0.5 text-xs rounded-full font-medium ${
              lead.match_score >= 80 ? 'bg-emerald-500/20 text-emerald-400' :
              lead.match_score >= 50 ? 'bg-amber-500/20 text-amber-400' :
              'bg-zinc-500/20 text-zinc-400'
            }`}
            data-testid="job-gate-score"
          >
            {lead.match_score}%
          </span>
        </div>
      </div>

      {/* Sections */}
      {SECTION_CONFIG.map(({ name, label, endpoint }) => {
        const section = sections[name];
        return (
          <div
            key={name}
            className="bg-elevated rounded-lg p-3 space-y-2"
            data-testid={`job-gate-section-${name}`}
          >
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-medium text-fg">{label}</h4>
              {!section.content && !section.loading && (
                <button
                  onClick={() => handleGenerate(name, endpoint)}
                  className="px-2.5 py-1 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
                  data-testid={`job-gate-generate-${name}`}
                >
                  Generate
                </button>
              )}
            </div>
            {section.loading && (
              <div className="flex items-center gap-2 text-sm text-fg-muted">
                <div className="w-3 h-3 border-2 border-soul/30 border-t-soul rounded-full animate-spin" />
                Generating...
              </div>
            )}
            {section.error && (
              <div className="text-sm text-red-400 bg-red-500/10 rounded px-3 py-2" data-testid={`job-gate-error-${name}`}>
                {section.error}
              </div>
            )}
            {section.content && (
              <pre
                className="text-sm text-fg whitespace-pre-wrap bg-deep rounded px-3 py-2 overflow-auto max-h-64"
                data-testid={`job-gate-content-${name}`}
              >
                {section.content}
              </pre>
            )}
          </div>
        );
      })}

      {/* Footer Actions */}
      <div className="flex items-center justify-end gap-2 pt-2 border-t border-border-subtle">
        <button
          onClick={() => onAction('skip', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-zinc-500/20 text-zinc-400 hover:bg-zinc-500/30 transition-colors"
          data-testid="job-gate-skip"
        >
          Skip
        </button>
        <button
          onClick={() => onAction('edit', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 transition-colors"
          data-testid="job-gate-edit"
        >
          Edit
        </button>
        <button
          onClick={() => onAction('approve', lead.id)}
          className="px-4 py-1.5 text-sm rounded bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors"
          data-testid="job-gate-approve"
        >
          Approve &amp; Send
        </button>
      </div>
    </div>
  );
}
