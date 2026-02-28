import type { ScoutPlatformSync } from '../../../lib/types.ts';

interface SyncStatusProps {
  platforms: ScoutPlatformSync[];
  lastRun: string;
}

function formatTime(iso: string): string {
  if (!iso) return 'never';
  const d = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - d.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  return `${Math.floor(diffHr / 24)}d ago`;
}

export default function SyncStatus({ platforms, lastRun }: SyncStatusProps) {
  if (!platforms || platforms.length === 0) return null;

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-fg-muted font-display">
          Platform Sync
        </h3>
        <span className="text-[10px] text-fg-muted">{formatTime(lastRun)}</span>
      </div>

      <div className="grid grid-cols-2 gap-2">
        {platforms.map((p) => {
          const synced = p.status === 'synced';
          return (
            <div
              key={p.platform}
              className={`rounded-lg px-3 py-2 ${
                synced
                  ? 'bg-emerald-500/10 border border-emerald-500/20'
                  : 'bg-amber-500/10 border border-amber-500/20'
              }`}
            >
              <div className="flex items-center gap-1.5">
                <span
                  className={`w-1.5 h-1.5 rounded-full ${
                    synced ? 'bg-emerald-400' : 'bg-amber-400'
                  }`}
                />
                <span className="text-xs font-medium text-fg capitalize">
                  {p.platform}
                </span>
              </div>
              {!synced && p.issues && p.issues.length > 0 && (
                <p className="text-[10px] text-amber-400/80 mt-1 truncate">
                  {p.issues[0]}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
