import { useState } from 'react';
import type { ScoutLead } from '../../hooks/useScout';
import { api } from '../../lib/api';

interface ContractGateProps {
  lead: ScoutLead;
  onAction: (action: string, leadId: number) => void;
}

interface SOWResult {
  scope: string;
  deliverables: string[];
  timeline: string;
  pricing: string;
}

interface FollowUpResult {
  message: string;
}

interface CaseStudyResult {
  title: string;
  challenge: string;
  approach: string;
  results: string;
}

interface UpsellResult {
  upsell_score: number;
  opportunities: string[];
  urgency: string;
}

type SectionKey = 'sow' | 'followup' | 'case-study' | 'upsell';

export function ContractGate({ lead, onAction }: ContractGateProps) {
  const [sow, setSOW] = useState<SOWResult | null>(null);
  const [followUp, setFollowUp] = useState<FollowUpResult | null>(null);
  const [caseStudy, setCaseStudy] = useState<CaseStudyResult | null>(null);
  const [upsell, setUpsell] = useState<UpsellResult | null>(null);
  const [loading, setLoading] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<SectionKey>>(new Set(['sow']));

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
        case 'sow': {
          const result = await api.post<SOWResult>('/api/ai/sow', { lead_id: lead.id });
          setSOW(result);
          break;
        }
        case 'followup': {
          const result = await api.post<FollowUpResult>('/api/ai/contract-followup', { lead_id: lead.id });
          setFollowUp(result);
          break;
        }
        case 'case-study': {
          const result = await api.post<CaseStudyResult>('/api/ai/case-study', { lead_id: lead.id });
          setCaseStudy(result);
          break;
        }
        case 'upsell': {
          const result = await api.post<UpsellResult>('/api/ai/contract-upsell', { lead_id: lead.id });
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

  const urgencyBadgeColor = (urgency: string) => {
    switch (urgency.toLowerCase()) {
      case 'high': return 'bg-red-500/20 text-red-400';
      case 'medium': return 'bg-amber-500/20 text-amber-400';
      default: return 'bg-emerald-500/20 text-emerald-400';
    }
  };

  const scoreColor = (score: number) => {
    if (score >= 80) return 'text-emerald-400';
    if (score >= 50) return 'text-amber-400';
    return 'text-red-400';
  };

  return (
    <div className="space-y-4" data-testid="contract-gate">
      {/* Header */}
      <div className="bg-surface rounded-lg p-4" data-testid="contract-gate-header">
        <div className="flex items-center justify-between mb-1">
          <h3 className="text-sm font-semibold text-fg">{lead.company}</h3>
          <span className="px-2 py-0.5 text-xs rounded-full bg-soul/20 text-soul capitalize">{lead.stage}</span>
        </div>
        <p className="text-xs text-fg-muted">{lead.title}</p>
      </div>

      {error && (
        <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="contract-gate-error">
          {error}
        </div>
      )}

      {/* SOW */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('sow')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="contract-section-sow"
        >
          <span>Statement of Work</span>
          <span className="text-fg-muted text-xs">{expanded.has('sow') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('sow') && (
          <div className="px-4 pb-4 space-y-3">
            {!sow ? (
              <button
                onClick={() => handleGenerate('sow')}
                disabled={loading === 'sow'}
                className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="contract-generate-sow-btn"
              >
                {loading === 'sow' ? 'Generating...' : 'Generate SOW'}
              </button>
            ) : (
              <div className="space-y-3" data-testid="contract-sow-result">
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Scope</h4>
                  <p className="text-sm text-fg">{sow.scope}</p>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Deliverables</h4>
                  <ul className="space-y-1">
                    {sow.deliverables.map((d, i) => (
                      <li key={i} className="text-sm text-fg flex items-start gap-2">
                        <span className="text-soul text-xs mt-0.5">&#x2022;</span>
                        <span>{d}</span>
                      </li>
                    ))}
                  </ul>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <h4 className="text-xs text-fg-muted font-medium mb-1">Timeline</h4>
                    <p className="text-sm text-fg font-medium">{sow.timeline}</p>
                  </div>
                  <div>
                    <h4 className="text-xs text-fg-muted font-medium mb-1">Pricing</h4>
                    <p className="text-sm text-fg font-medium">{sow.pricing}</p>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Follow Up */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('followup')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="contract-section-followup"
        >
          <span>Follow Up</span>
          <span className="text-fg-muted text-xs">{expanded.has('followup') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('followup') && (
          <div className="px-4 pb-4 space-y-3">
            {!followUp ? (
              <button
                onClick={() => handleGenerate('followup')}
                disabled={loading === 'followup'}
                className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="contract-generate-followup-btn"
              >
                {loading === 'followup' ? 'Generating...' : 'Generate Follow Up'}
              </button>
            ) : (
              <div data-testid="contract-followup-result">
                <pre className="text-sm text-fg whitespace-pre-wrap font-sans bg-elevated rounded-lg p-3">{followUp.message}</pre>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Case Study */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('case-study')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="contract-section-case-study"
        >
          <span>Case Study</span>
          <span className="text-fg-muted text-xs">{expanded.has('case-study') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('case-study') && (
          <div className="px-4 pb-4 space-y-3">
            {!caseStudy ? (
              <button
                onClick={() => handleGenerate('case-study')}
                disabled={loading === 'case-study'}
                className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="contract-generate-case-study-btn"
              >
                {loading === 'case-study' ? 'Generating...' : 'Generate Case Study'}
              </button>
            ) : (
              <div className="space-y-3" data-testid="contract-case-study-result">
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Title</h4>
                  <p className="text-sm text-fg font-medium">{caseStudy.title}</p>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Challenge</h4>
                  <p className="text-sm text-fg">{caseStudy.challenge}</p>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Approach</h4>
                  <p className="text-sm text-fg">{caseStudy.approach}</p>
                </div>
                <div>
                  <h4 className="text-xs text-fg-muted font-medium mb-1">Results</h4>
                  <p className="text-sm text-fg">{caseStudy.results}</p>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Upsell Detection */}
      <div className="bg-surface rounded-lg overflow-hidden">
        <button
          onClick={() => toggle('upsell')}
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-fg hover:bg-elevated transition-colors"
          data-testid="contract-section-upsell"
        >
          <span>Upsell Detection</span>
          <span className="text-fg-muted text-xs">{expanded.has('upsell') ? 'Collapse' : 'Expand'}</span>
        </button>
        {expanded.has('upsell') && (
          <div className="px-4 pb-4 space-y-3">
            {!upsell ? (
              <button
                onClick={() => handleGenerate('upsell')}
                disabled={loading === 'upsell'}
                className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 disabled:opacity-50 transition-colors"
                data-testid="contract-generate-upsell-btn"
              >
                {loading === 'upsell' ? 'Generating...' : 'Detect Upsell'}
              </button>
            ) : (
              <div className="space-y-3" data-testid="contract-upsell-result">
                <div className="flex items-center gap-3">
                  {/* Score gauge */}
                  <div className="relative w-16 h-16">
                    <svg viewBox="0 0 36 36" className="w-16 h-16 -rotate-90">
                      <circle cx="18" cy="18" r="15.5" fill="none" stroke="currentColor" strokeWidth="2" className="text-elevated" />
                      <circle
                        cx="18" cy="18" r="15.5" fill="none" strokeWidth="2"
                        strokeDasharray={`${upsell.upsell_score * 0.975} 97.5`}
                        strokeLinecap="round"
                        className={scoreColor(upsell.upsell_score)}
                        stroke="currentColor"
                      />
                    </svg>
                    <span className={`absolute inset-0 flex items-center justify-center text-sm font-bold ${scoreColor(upsell.upsell_score)}`}>
                      {upsell.upsell_score}
                    </span>
                  </div>
                  <div>
                    <div className="text-sm font-medium text-fg">Upsell Score</div>
                    <span className={`inline-block mt-1 px-2 py-0.5 text-xs rounded-full ${urgencyBadgeColor(upsell.urgency)}`}>
                      {upsell.urgency} urgency
                    </span>
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
          onClick={() => onAction('send-sow', lead.id)}
          className="px-4 py-1.5 text-xs rounded bg-soul text-deep font-medium hover:bg-soul/90 transition-colors"
          data-testid="contract-send-sow-btn"
        >
          Send SOW
        </button>
        <button
          onClick={() => onAction('follow-up', lead.id)}
          className="px-3 py-1.5 text-xs rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors"
          data-testid="contract-follow-up-btn"
        >
          Follow Up
        </button>
        <button
          onClick={() => onAction('skip', lead.id)}
          className="px-3 py-1.5 text-xs rounded bg-elevated text-fg-secondary hover:bg-overlay transition-colors"
          data-testid="contract-skip-btn"
        >
          Skip
        </button>
      </div>
    </div>
  );
}
