import type { ScoutMetric } from '../../../lib/types.ts';

interface WeeklyMetricsProps {
  metrics: Record<string, ScoutMetric>;
}

export default function WeeklyMetrics({ metrics }: WeeklyMetricsProps) {
  const entries = Object.entries(metrics)
    .sort(([a], [b]) => b.localeCompare(a))
    .slice(0, 4);

  if (entries.length === 0) return null;

  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted font-display">
        Weekly Metrics
      </h3>

      <div className="space-y-1.5">
        {entries.map(([week, m]) => (
          <div
            key={week}
            className="rounded-lg bg-elevated/50 border border-border-subtle px-3 py-2"
          >
            <div className="text-[10px] text-fg-muted mb-1 font-mono">{week}</div>
            <div className="flex items-center gap-3">
              <Stat label="Applied" value={m.applied} color="text-blue-400" />
              <Stat label="Responses" value={m.responses} color="text-cyan-400" />
              <Stat label="Interviews" value={m.interviews} color="text-purple-400" />
              <Stat label="Offers" value={m.offers} color="text-emerald-400" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function Stat({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="flex flex-col items-center min-w-0">
      <span className={`text-sm font-semibold ${color}`}>{value}</span>
      <span className="text-[10px] text-fg-muted truncate">{label}</span>
    </div>
  );
}
