import type { ScoutScoredLead } from '../../hooks/useScout';

interface HotLeadsTableProps {
  leads: ScoutScoredLead[];
}

export function HotLeadsTable({ leads }: HotLeadsTableProps) {
  const sorted = [...leads].sort((a, b) => b.match_score - a.match_score);

  return (
    <div className="bg-surface rounded-lg p-4" data-testid="hot-leads-table">
      <h3 className="text-sm font-medium text-fg-muted mb-3">Hot Leads</h3>
      {sorted.length === 0 ? (
        <div className="text-xs text-fg-muted">No scored leads available</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-fg-muted border-b border-border-subtle">
                <th className="text-left py-1.5 pr-3">Title</th>
                <th className="text-left py-1.5 pr-3">Company</th>
                <th className="text-left py-1.5 pr-3">Type</th>
                <th className="text-left py-1.5 pr-3">Score</th>
                <th className="text-left py-1.5">Stage</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map(lead => (
                <tr key={lead.id} className="border-b border-border-subtle/50" data-testid={`hot-lead-row-${lead.id}`}>
                  <td className="py-1.5 pr-3 text-fg">{lead.title}</td>
                  <td className="py-1.5 pr-3 text-fg-muted">{lead.company}</td>
                  <td className="py-1.5 pr-3 text-fg-muted capitalize">{lead.type}</td>
                  <td className="py-1.5 pr-3">
                    <span className={`font-medium ${lead.match_score >= 80 ? 'text-emerald-400' : lead.match_score >= 50 ? 'text-amber-400' : 'text-fg-muted'}`}>
                      {lead.match_score}%
                    </span>
                  </td>
                  <td className="py-1.5 text-fg-muted capitalize">{lead.stage}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
