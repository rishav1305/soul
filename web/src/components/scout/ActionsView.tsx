import type { ScoutLead, ScoutOptimization } from '../../hooks/useScout';

interface ActionsViewProps {
  leads: ScoutLead[];
  optimizations: ScoutOptimization[];
  onApproveOptimization: (id: number) => void;
  onRejectOptimization: (id: number) => void;
}

function isStale(updatedAt: string): boolean {
  const diff = Date.now() - new Date(updatedAt).getTime();
  return diff > 14 * 24 * 60 * 60 * 1000; // 14 days
}

function isDueFollowUp(updatedAt: string): boolean {
  const diff = Date.now() - new Date(updatedAt).getTime();
  return diff > 7 * 24 * 60 * 60 * 1000; // 7 days
}

export function ActionsView({ leads, optimizations, onApproveOptimization, onRejectOptimization }: ActionsViewProps) {
  const activeLeads = leads.filter(l => !['closed', 'rejected'].includes(l.stage));
  const followUps = activeLeads.filter(l => isDueFollowUp(l.updated_at));
  const staleLeads = activeLeads.filter(l => isStale(l.updated_at));
  const pendingOptimizations = optimizations.filter(o => o.status === 'pending');

  // Pipeline gaps: stages with zero leads
  const stagesWithLeads = new Set(activeLeads.map(l => l.stage));
  const expectedStages = ['discovered', 'applied', 'screening', 'interviewing', 'negotiating'];
  const gapStages = expectedStages.filter(s => !stagesWithLeads.has(s));

  // Source effectiveness
  const sourceMap = new Map<string, { total: number; closed: number }>();
  for (const lead of leads) {
    const entry = sourceMap.get(lead.source) ?? { total: 0, closed: 0 };
    entry.total++;
    if (lead.stage === 'closed') entry.closed++;
    sourceMap.set(lead.source, entry);
  }

  return (
    <div className="space-y-6" data-testid="actions-view">
      {/* Follow-ups due */}
      <div className="bg-surface rounded-lg p-4" data-testid="actions-followups">
        <h3 className="text-sm font-medium text-fg-muted mb-3">Follow-ups Due ({followUps.length})</h3>
        {followUps.length === 0 ? (
          <div className="text-xs text-fg-muted">No follow-ups due</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-fg-muted border-b border-border-subtle">
                  <th className="text-left py-1 pr-3">Title</th>
                  <th className="text-left py-1 pr-3">Company</th>
                  <th className="text-left py-1 pr-3">Stage</th>
                  <th className="text-left py-1">Last Updated</th>
                </tr>
              </thead>
              <tbody>
                {followUps.map(lead => (
                  <tr key={lead.id} className="border-b border-border-subtle/50" data-testid={`followup-row-${lead.id}`}>
                    <td className="py-1.5 pr-3 text-fg">{lead.title}</td>
                    <td className="py-1.5 pr-3 text-fg-muted">{lead.company}</td>
                    <td className="py-1.5 pr-3 capitalize text-fg-muted">{lead.stage}</td>
                    <td className="py-1.5 text-fg-muted">{new Date(lead.updated_at).toLocaleDateString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Stale leads */}
      <div className="bg-surface rounded-lg p-4" data-testid="actions-stale">
        <h3 className="text-sm font-medium text-fg-muted mb-3">Stale Leads ({staleLeads.length})</h3>
        {staleLeads.length === 0 ? (
          <div className="text-xs text-fg-muted">No stale leads</div>
        ) : (
          <div className="space-y-1.5">
            {staleLeads.map(lead => (
              <div key={lead.id} className="flex items-center justify-between text-sm" data-testid={`stale-lead-${lead.id}`}>
                <span className="text-fg">{lead.title} <span className="text-fg-muted">at {lead.company}</span></span>
                <span className="text-xs text-red-400">{lead.stage}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Pipeline gaps */}
      <div className="bg-surface rounded-lg p-4" data-testid="actions-gaps">
        <h3 className="text-sm font-medium text-fg-muted mb-3">Pipeline Gaps</h3>
        {gapStages.length === 0 ? (
          <div className="text-xs text-emerald-400">All stages covered</div>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {gapStages.map(stage => (
              <span key={stage} className="px-2 py-0.5 text-xs rounded-full bg-amber-500/20 text-amber-400 capitalize">{stage}</span>
            ))}
          </div>
        )}
      </div>

      {/* Source effectiveness */}
      <div className="bg-surface rounded-lg p-4" data-testid="actions-source-effectiveness">
        <h3 className="text-sm font-medium text-fg-muted mb-3">Source Effectiveness</h3>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-fg-muted border-b border-border-subtle">
                <th className="text-left py-1 pr-3">Source</th>
                <th className="text-left py-1 pr-3">Total</th>
                <th className="text-left py-1 pr-3">Closed</th>
                <th className="text-left py-1">Rate</th>
              </tr>
            </thead>
            <tbody>
              {Array.from(sourceMap.entries()).map(([source, data]) => (
                <tr key={source} className="border-b border-border-subtle/50">
                  <td className="py-1.5 pr-3 text-fg">{source}</td>
                  <td className="py-1.5 pr-3 text-fg-muted">{data.total}</td>
                  <td className="py-1.5 pr-3 text-fg-muted">{data.closed}</td>
                  <td className="py-1.5 text-fg-muted">{data.total > 0 ? Math.round((data.closed / data.total) * 100) : 0}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pending optimizations */}
      {pendingOptimizations.length > 0 && (
        <div className="bg-surface rounded-lg p-4" data-testid="actions-optimizations">
          <h3 className="text-sm font-medium text-fg-muted mb-3">Pending Optimizations ({pendingOptimizations.length})</h3>
          <div className="space-y-3">
            {pendingOptimizations.map(opt => (
              <div key={opt.id} className="bg-elevated rounded-lg p-3 space-y-2" data-testid={`optimization-${opt.id}`}>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-fg font-medium">{opt.field}</span>
                  <span className="text-xs text-fg-muted capitalize">{opt.type}</span>
                </div>
                <p className="text-xs text-fg-muted">{opt.reason}</p>
                <div className="flex gap-2">
                  <button
                    onClick={() => onApproveOptimization(opt.id)}
                    className="px-2 py-0.5 text-xs rounded bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors"
                    data-testid={`approve-opt-${opt.id}`}
                  >
                    Approve
                  </button>
                  <button
                    onClick={() => onRejectOptimization(opt.id)}
                    className="px-2 py-0.5 text-xs rounded bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-colors"
                    data-testid={`reject-opt-${opt.id}`}
                  >
                    Reject
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
