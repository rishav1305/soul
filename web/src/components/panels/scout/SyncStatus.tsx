import { useState } from 'react';
import type { ScoutPlatformSync, ScoutSyncDetail } from '../../../lib/types.ts';

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

function FieldDetail({ detail }: { detail: ScoutSyncDetail }) {
  return (
    <div className="flex items-start gap-1.5 text-[10px]">
      <span className={detail.match ? 'text-emerald-400' : 'text-red-400'}>
        {detail.match ? '\u2713' : '\u2717'}
      </span>
      <div className="min-w-0 flex-1">
        <span className="font-medium text-fg capitalize">{detail.field}</span>
        <span className="text-fg-muted ml-1 truncate block">{detail.expected}</span>
      </div>
    </div>
  );
}

function PlatformCard({ platform }: { platform: ScoutPlatformSync }) {
  const [expanded, setExpanded] = useState(false);
  const synced = platform.status === 'synced';
  const hasDetails = platform.details && platform.details.length > 0;

  // Count matched/total fields.
  const matched = platform.details?.filter((d) => d.match).length ?? 0;
  const total = platform.details?.length ?? 0;

  return (
    <div
      className={`rounded-lg px-3 py-2 ${
        synced
          ? 'bg-emerald-500/10 border border-emerald-500/20'
          : 'bg-amber-500/10 border border-amber-500/20'
      }`}
    >
      <div
        className={`flex items-center justify-between ${hasDetails ? 'cursor-pointer' : ''}`}
        onClick={() => hasDetails && setExpanded(!expanded)}
        onKeyDown={(e) => e.key === 'Enter' && hasDetails && setExpanded(!expanded)}
        role={hasDetails ? 'button' : undefined}
        tabIndex={hasDetails ? 0 : undefined}
      >
        <div className="flex items-center gap-1.5">
          <span
            className={`w-1.5 h-1.5 rounded-full ${
              synced ? 'bg-emerald-400' : 'bg-amber-400'
            }`}
          />
          <span className="text-xs font-medium text-fg capitalize">
            {platform.platform}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          {total > 0 && (
            <span className={`text-[10px] font-medium ${synced ? 'text-emerald-400' : 'text-amber-400'}`}>
              {matched}/{total}
            </span>
          )}
          {hasDetails && (
            <span className="text-[10px] text-fg-muted">
              {expanded ? '\u25B4' : '\u25BE'}
            </span>
          )}
        </div>
      </div>

      {/* Issue summary when collapsed */}
      {!expanded && !synced && platform.issues && platform.issues.length > 0 && (
        <p className="text-[10px] text-amber-400/80 mt-1 truncate">
          {platform.issues[0]}
        </p>
      )}

      {/* Expanded field details */}
      {expanded && hasDetails && (
        <div className="mt-2 space-y-1 border-t border-white/5 pt-2">
          {platform.details!.map((d) => (
            <FieldDetail key={d.field} detail={d} />
          ))}
        </div>
      )}
    </div>
  );
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
        {platforms.map((p) => (
          <PlatformCard key={p.platform} platform={p} />
        ))}
      </div>
    </div>
  );
}
