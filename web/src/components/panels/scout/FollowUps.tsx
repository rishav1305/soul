import type { ScoutFollowUp } from '../../../lib/types.ts';

interface FollowUpsProps {
  followUps: ScoutFollowUp[];
}

function formatDue(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  const now = new Date();
  const diffMs = d.getTime() - now.getTime();
  const diffDays = Math.floor(diffMs / 86400000);
  if (diffDays < -1) return `${Math.abs(diffDays)}d overdue`;
  if (diffDays === -1) return 'yesterday';
  if (diffDays === 0) return 'today';
  if (diffDays === 1) return 'tomorrow';
  return `in ${diffDays}d`;
}

export default function FollowUps({ followUps }: FollowUpsProps) {
  if (!followUps || followUps.length === 0) return null;

  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted font-display">
        Follow-ups
      </h3>

      <div className="space-y-1.5">
        {followUps.map((fu, i) => (
          <div
            key={`${fu.company}-${fu.role}-${i}`}
            className="rounded-lg bg-amber-500/8 border border-amber-500/20 px-3 py-2 space-y-0.5"
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-fg truncate">
                {fu.company} &mdash; {fu.role}
              </span>
              <span className="shrink-0 text-[10px] font-medium text-amber-400">
                {formatDue(fu.follow_up)}
              </span>
            </div>
            <p className="text-[10px] text-amber-300/70 truncate">{fu.notes}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
