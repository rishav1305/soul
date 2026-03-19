import { useState } from 'react';
import type { ScoutLead } from '../../hooks/useScout';
import { api } from '../../lib/api';

interface ConsultingGateProps {
  lead: ScoutLead;
  onAction: (action: string, leadId: number) => void;
}

interface CallPrepResult {
  company_background: string;
  likely_questions: string[];
  relevant_experience: string[];
}

interface AdvisoryProposalResult {
  executive_summary: string;
  scope: string;
  deliverables: string[];
  pricing: string;
}

interface ProjectProposalResult {
  milestones: { name: string; duration: string; description: string }[];
  budget: string;
  timeline: string;
}

interface UpsellResult {
  score: number;
  opportunities: string[];
}

type SectionKey = 'call-prep' | 'advisory' | 'project' | 'upsell';

export function ConsultingGate({ lead, onAction }: ConsultingGateProps) {
  const [callPrep, setCallPrep] = useState<CallPrepResult | null>(null);
  const [advisory, setAdvisory] = useState<AdvisoryProposalResult | null>(null);
  const [project, setProject] = useState<ProjectProposalResult | null>(null);
  const [upsell, setUpsell] = useState<UpsellResult | null>(null);
  const [loading, setLoading] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<SectionKey>>(new Set(['call-prep']));

  const toggle = (section: SectionKey) => {
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(section)) {
        next.delete(section);
      } else {
        next.add(section);
      }
      return next;
    });
  };

  const handleGenerate = async (section: SectionKey) => {
    setLoading(section);
    setError(null);
    try {
      switch (section) {
        case 'call-prep': {
          const result = await api.post<CallPrepResult>('/api/ai/call-prep', { lead_id: lead.id });
          setCallPrep(result);
          break;
        }
        case 'advisory': {
          const result = await api.post<AdvisoryProposalResult>('/api/ai/advisory-proposal', { lead_id: lead.id });
          setAdvisory(result);
          break;
        }
        case 'project': {
          const result = await api.post<ProjectProposalResult>('/api/ai/project-proposal', { lead_id: lead.id });
          setProject(result);
          break;
        }
        case 'upsell': {
          const result = await api.post<UpsellResult>('/api/ai/consulting-upsell', { lead_id: lead.id });
          setUpsell(result);
          break;
        }
      }
      setExpanded(prev => new Set(prev).add(section));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(null);
    }
  };

  const scoreColor = (score: number) => {
    if (score >= 80) return 'text-emerald-400';
    if (score >= 50) return 'text-amber-400';
    return 'text-red-400';
  };

  return (
    <div className="space-y-4" data-testid="consulting-gate">
      {/* Header */}
      <div className="bg-surface rounded-lg p-4" data-testid="consulting-gate-header">
        <div className="flex items-center justify-between mb-1">
          <h3 className="text-sm font-semibold text-fg">{lead.company}</h3>
          <span className="px-2 py-0.5 text-xs rounded-full bg-soul/20 text-soul capitalize">{lead.stage}</span>
        </div>
        <p className="text-xs text-fg-muted line-clamp-2">{lead.notes || lead.title}</p>
      </div>

      {error && (
        <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="consulting-gate-error">
          {error}
        </div>
      )}

      {/* Call Prep */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('call-prep')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="consulting-section-call-prep"
        >
          <span>Call Prep</span>
          <span className="text-fg-muted text-xs">{expanded.has('call-prep') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('call-prep') && (
          <div className="px-4 pb-4 space-y-3">
            {!callPrep ? (
              <button
                onClick={() => handleGenerate('call-prep')}
                disabled={loading === 'call-prep'}
                className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="consulting-generate-call-prep-btn"
              >
                {loading === 'call-prep' ? 'Generating...' : 'Generate Call Prep'}
              </button>
            ) : (
              <div className="space-y-3" data-testid="consulting-call-prep-result">
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Company Background</h4>
                  <p className="text-sm text-fg">{callPrep.company_background}</p>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Likely Questions</h4>
                  <ul className="space-y-1">
                    {callPrep.likely_questions.map((q, i) => (
                      <li key={i} className="text-sm text-fg flex items-start gap-2">
                        <span className="text-soul text-xs mt-0.5">&#x2022;</span>
                        <span>{q}</span>
                      </li>
                    ))}
                  </ul>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Relevant Experience</h4>
                  <ul className="space-y-1">
                    {callPrep.relevant_experience.map((exp, i) => (
                      <li key={i} className="text-sm text-fg flex items-start gap-2">
                        <span className="text-soul text-xs mt-0.5">&#x2022;</span>
                        <span>{exp}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Advisory Proposal */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('advisory')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="consulting-section-advisory"
        >
          <span>Advisory Proposal</span>
          <span className="text-fg-muted text-xs">{expanded.has('advisory') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('advisory') && (
          <div className="px-4 pb-4 space-y-3">
            {!advisory ? (
              <button
                onClick={() => handleGenerate('advisory')}
                disabled={loading === 'advisory'}
                className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="consulting-generate-advisory-btn"
              >
                {loading === 'advisory' ? 'Generating...' : 'Generate Advisory Proposal'}
              </button>
            ) : (
              <div className="space-y-3" data-testid="consulting-advisory-result">
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Executive Summary</h4>
                  <p className="text-sm text-fg">{advisory.executive_summary}</p>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Scope</h4>
                  <p className="text-sm text-fg">{advisory.scope}</p>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Deliverables</h4>
                  <ul className="space-y-1">
                    {advisory.deliverables.map((d, i) => (
                      <li key={i} className="text-sm text-fg flex items-start gap-2">
                        <span className="text-soul text-xs mt-0.5">&#x2022;</span>
                        <span>{d}</span>
                      </li>
                    ))}
                  </ul>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Pricing</h4>
                  <p className="text-sm text-fg font-medium">{advisory.pricing}</p>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Project Proposal */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('project')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="consulting-section-project"
        >
          <span>Project Proposal</span>
          <span className="text-fg-muted text-xs">{expanded.has('project') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('project') && (
          <div className="px-4 pb-4 space-y-3">
            {!project ? (
              <button
                onClick={() => handleGenerate('project')}
                disabled={loading === 'project'}
                className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="consulting-generate-project-btn"
              >
                {loading === 'project' ? 'Generating...' : 'Generate Project Proposal'}
              </button>
            ) : (
              <div className="space-y-3" data-testid="consulting-project-result">
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Milestones</h4>
                  <div className="space-y-2">
                    {project.milestones.map((ms, i) => (
                      <div key={i} className="bg-deep rounded p-2 border-l-2 border-soul/30">
                        <div className="text-sm font-medium text-fg">{ms.name}</div>
                        <div className="text-xs text-fg-muted">{ms.duration}</div>
                        <p className="text-xs text-fg-secondary mt-0.5">{ms.description}</p>
                      </div>
                    ))}
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <h4 className="text-xs text-fg-muted font-medium mb-1">Budget</h4>
                    <p className="text-sm text-fg font-medium">{project.budget}</p>
                  </div>
                  <div>
                    <h4 className="text-xs text-fg-muted font-medium mb-1">Timeline</h4>
                    <p className="text-sm text-fg font-medium">{project.timeline}</p>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Upsell Evaluation */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('upsell')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="consulting-section-upsell"
        >
          <span>Upsell Evaluation</span>
          <span className="text-fg-muted text-xs">{expanded.has('upsell') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('upsell') && (
          <div className="px-4 pb-4 space-y-3">
            {!upsell ? (
              <button
                onClick={() => handleGenerate('upsell')}
                disabled={loading === 'upsell'}
                className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="consulting-generate-upsell-btn"
              >
                {loading === 'upsell' ? 'Generating...' : 'Evaluate Upsell'}
              </button>
            ) : (
              <div className="space-y-3" data-testid="consulting-upsell-result">
                {/* Score gauge */}
                <div className="flex items-center gap-3">
                  <div className="relative w-16 h-16">
                    <svg viewBox="0 0 36 36" className="w-16 h-16 -rotate-90">
                      <circle cx="18" cy="18" r="15.5" fill="none" stroke="currentColor" strokeWidth="2" className="text-elevated" />
                      <circle
                        cx="18" cy="18" r="15.5" fill="none" strokeWidth="2"
                        strokeDasharray={`${upsell.score * 0.975} 97.5`}
                        strokeLinecap="round"
                        className={scoreColor(upsell.score)}
                        stroke="currentColor"
                      />
                    </svg>
                    <span className={`absolute inset-0 flex items-center justify-center text-sm font-bold ${scoreColor(upsell.score)}`}>
                      {upsell.score}
                    </span>
                  </div>
                  <div>
                    <div className="text-sm font-medium text-fg">Upsell Score</div>
                    <div className="text-xs text-fg-muted">
                      {upsell.score >= 80 ? 'High potential' : upsell.score >= 50 ? 'Moderate potential' : 'Low potential'}
                    </div>
                  </div>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Opportunities</h4>
                  <ul className="space-y-1">
                    {upsell.opportunities.map((opp, i) => (
                      <li key={i} className="text-sm text-fg flex items-start gap-2">
                        <span className="text-soul text-xs mt-0.5">&#x2022;</span>
                        <span>{opp}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="flex items-center gap-2 pt-2 border-t border-border-subtle">
        <button
          onClick={() => onAction('send-proposal', lead.id)}
          className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 transition-colors"
          data-testid="consulting-send-proposal-btn"
        >
          Send Proposal
        </button>
        <button
          onClick={() => onAction('follow-up', lead.id)}
          className="px-3 py-1.5 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
          data-testid="consulting-follow-up-btn"
        >
          Follow Up
        </button>
        <button
          onClick={() => onAction('skip', lead.id)}
          className="px-3 py-1.5 text-xs rounded bg-elevated text-fg-secondary hover:bg-overlay transition-colors"
          data-testid="consulting-skip-btn"
        >
          Skip
        </button>
      </div>
    </div>
  );
}
