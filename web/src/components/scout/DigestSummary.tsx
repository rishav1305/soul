import type { ScoutLead, ScoutAnalytics } from '../../hooks/useScout';

interface DigestSummaryProps {
  leads: ScoutLead[];
  analytics: ScoutAnalytics | null;
}

export function DigestSummary({ leads, analytics }: DigestSummaryProps) {
  const oneWeekAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
  const newLeads = leads.filter(l => new Date(l.created_at).getTime() > oneWeekAgo);
  const followUpsDue = leads.filter(l => {
    const diff = Date.now() - new Date(l.updated_at).getTime();
    return diff > 7 * 24 * 60 * 60 * 1000 && !['closed', 'rejected'].includes(l.stage);
  });

  const topSources = analytics
    ? Object.entries(analytics.by_source)
        .sort(([, a], [, b]) => b - a)
        .slice(0, 3)
    : [];

  return (
    <div className="bg-surface rounded-lg p-4 space-y-4" data-testid="digest-summary">
      <h3 className="text-sm font-medium text-fg-muted">Weekly Digest</h3>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="text-center" data-testid="digest-new-leads">
          <div className="text-2xl font-bold text-fg">{newLeads.length}</div>
          <div className="text-xs text-fg-muted">New Leads</div>
        </div>
        <div className="text-center" data-testid="digest-active">
          <div className="text-2xl font-bold text-emerald-400">{analytics?.active_leads ?? 0}</div>
          <div className="text-xs text-fg-muted">Active</div>
        </div>
        <div className="text-center" data-testid="digest-followups">
          <div className={`text-2xl font-bold ${followUpsDue.length > 0 ? 'text-amber-400' : 'text-fg'}`}>{followUpsDue.length}</div>
          <div className="text-xs text-fg-muted">Follow-ups Due</div>
        </div>
        <div className="text-center" data-testid="digest-total">
          <div className="text-2xl font-bold text-fg">{analytics?.total_leads ?? leads.length}</div>
          <div className="text-xs text-fg-muted">Total</div>
        </div>
      </div>

      {/* Top sources */}
      {topSources.length > 0 && (
        <div>
          <h4 className="text-xs text-fg-muted mb-2">Top Sources</h4>
          <div className="flex flex-wrap gap-1.5">
            {topSources.map(([source, count]) => (
              <span key={source} className="px-2 py-0.5 text-xs rounded-full bg-soul/10 text-soul">
                {source} ({count})
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
