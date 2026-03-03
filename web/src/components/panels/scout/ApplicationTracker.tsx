import type { ScoutApplication } from '../../../lib/types.ts';

interface ApplicationTrackerProps {
  applications: ScoutApplication[];
  byStatus: Record<string, number>;
}

const STATUS_COLORS: Record<string, string> = {
  applied: 'bg-blue-500/15 text-blue-400 border-blue-500/25',
  viewed: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/25',
  interview_scheduled: 'bg-purple-500/15 text-purple-400 border-purple-500/25',
  interview_done: 'bg-violet-500/15 text-violet-400 border-violet-500/25',
  offer: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/25',
  rejected: 'bg-red-500/15 text-red-400 border-red-500/25',
  withdrawn: 'bg-zinc-500/15 text-zinc-400 border-zinc-500/25',
  follow_up_sent: 'bg-amber-500/15 text-amber-400 border-amber-500/25',
};

function statusLabel(status: string): string {
  return status.replace(/_/g, ' ');
}

export default function ApplicationTracker({ applications, byStatus }: ApplicationTrackerProps) {
  const statuses = Object.entries(byStatus ?? {}).filter(([, count]) => count > 0);
  const recent = (applications ?? []).slice(0, 10);

  if (recent.length === 0 && statuses.length === 0) return null;

  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted font-display">
        Applications
      </h3>

      {/* Status pills */}
      {statuses.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {statuses.map(([status, count]) => (
            <span
              key={status}
              className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium border capitalize ${
                STATUS_COLORS[status] ?? 'bg-zinc-500/15 text-zinc-400 border-zinc-500/25'
              }`}
            >
              {statusLabel(status)}
              <span className="opacity-70">{count}</span>
            </span>
          ))}
        </div>
      )}

      {/* Recent list */}
      {recent.length > 0 && (
        <div className="space-y-1">
          {recent.map((app) => (
            <div
              key={app.id}
              className="flex items-center gap-2 rounded-lg bg-elevated/50 border border-border-subtle px-3 py-2"
            >
              <div className="flex-1 min-w-0">
                <div className="text-xs font-medium text-fg truncate">{app.role}</div>
                <div className="text-[10px] text-fg-secondary truncate">
                  {app.company}
                </div>
              </div>
              <span
                className={`shrink-0 px-1.5 py-0.5 rounded text-[10px] font-medium capitalize border ${
                  STATUS_COLORS[app.status] ?? 'bg-zinc-500/15 text-zinc-400 border-zinc-500/25'
                }`}
              >
                {statusLabel(app.status)}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
