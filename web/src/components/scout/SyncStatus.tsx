interface PlatformStatus {
  platform: string;
  lastSync: string;
  status: 'healthy' | 'stale' | 'error';
}

const PLATFORMS: PlatformStatus[] = [
  { platform: 'LinkedIn', lastSync: '', status: 'stale' },
  { platform: 'GitHub', lastSync: '', status: 'stale' },
  { platform: 'Naukri', lastSync: '', status: 'stale' },
  { platform: 'Wellfound', lastSync: '', status: 'stale' },
];

const statusColor: Record<string, string> = {
  healthy: 'bg-emerald-500/20 text-emerald-400',
  stale: 'bg-amber-500/20 text-amber-400',
  error: 'bg-red-500/20 text-red-400',
};

const statusDot: Record<string, string> = {
  healthy: 'bg-emerald-400',
  stale: 'bg-amber-400',
  error: 'bg-red-400',
};

interface SyncStatusProps {
  onSync: (platform: string) => void;
}

export function SyncStatus({ onSync }: SyncStatusProps) {
  return (
    <div className="bg-surface rounded-lg p-4" data-testid="sync-status">
      <h3 className="text-sm font-medium text-fg-muted mb-3">Platform Sync</h3>
      <div className="space-y-2">
        {PLATFORMS.map(p => (
          <div key={p.platform} className="flex items-center justify-between" data-testid={`sync-platform-${p.platform.toLowerCase()}`}>
            <div className="flex items-center gap-2">
              <div className={`w-2 h-2 rounded-full ${statusDot[p.status]}`} />
              <span className="text-sm text-fg">{p.platform}</span>
            </div>
            <div className="flex items-center gap-2">
              <span className={`px-1.5 py-0.5 text-[10px] rounded-full ${statusColor[p.status]}`}>
                {p.status}
              </span>
              <button
                onClick={() => onSync(p.platform.toLowerCase())}
                className="px-2 py-0.5 text-xs rounded bg-elevated text-fg-muted hover:text-fg transition-colors"
                data-testid={`sync-btn-${p.platform.toLowerCase()}`}
              >
                Sync
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
