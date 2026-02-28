import type { ScoutOpportunity } from '../../../lib/types.ts';

interface OpportunitiesProps {
  opportunities: ScoutOpportunity[];
}

export default function Opportunities({ opportunities }: OpportunitiesProps) {
  const visible = opportunities.filter((o) => !o.dismissed);

  if (visible.length === 0) {
    return (
      <div className="space-y-2">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted font-display">
          Opportunities
        </h3>
        <p className="text-xs text-fg-muted italic">No new opportunities found.</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted font-display">
          Opportunities
        </h3>
        <span className="text-[10px] text-fg-muted">{visible.length} new</span>
      </div>

      <div className="space-y-1.5">
        {visible.map((opp) => (
          <div
            key={opp.id}
            className="rounded-lg bg-elevated/50 border border-border-subtle px-3 py-2 space-y-1"
          >
            <div className="flex items-start justify-between gap-2">
              <span className="text-xs font-medium text-fg truncate">{opp.role}</span>
              {opp.match && (
                <span className="shrink-0 text-[10px] font-medium text-soul bg-soul/10 rounded px-1.5 py-0.5">
                  {opp.match}
                </span>
              )}
            </div>
            <div className="flex items-center gap-2 text-[10px] text-fg-secondary">
              <span>{opp.company}</span>
              <span className="text-fg-muted">on</span>
              <span className="capitalize">{opp.platform}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
