import type { MeshNode, Heartbeat } from '../../hooks/useMesh';

const roleBadge: Record<string, string> = {
  hub: 'bg-soul-dim text-soul',
  agent: 'bg-blue-500/20 text-blue-400',
};

const statusDot: Record<string, string> = {
  online: 'bg-emerald-400',
  offline: 'bg-zinc-500',
  degraded: 'bg-amber-400',
};

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

/** Simple inline bar chart row for heartbeat metrics */
function MetricBar({ label, value, max, unit, color }: { label: string; value: number; max: number; unit: string; color: string }) {
  const pct = max > 0 ? Math.min(Math.round((value / max) * 100), 100) : 0;
  return (
    <div className="flex items-center gap-3 text-xs" data-testid={`metric-bar-${label.toLowerCase().replace(/\s+/g, '-')}`}>
      <span className="text-fg-muted w-10 shrink-0 text-right">{label}</span>
      <div className="flex-1 h-1.5 bg-elevated rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-fg-secondary font-mono w-16 text-right">{value}{unit}</span>
    </div>
  );
}

interface NodeDetailProps {
  node: MeshNode;
  heartbeats: Heartbeat[];
  onClose: () => void;
}

export function NodeDetail({ node, heartbeats, onClose }: NodeDetailProps) {
  const ramGb = (node.ram_total_mb / 1024).toFixed(1);

  return (
    <div className="space-y-4" data-testid={`node-detail-${node.id}`}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h3 className="text-base font-semibold text-fg">{node.name}</h3>
          <span className={`px-2 py-0.5 text-xs rounded-full ${roleBadge[node.role] ?? 'bg-overlay text-fg-secondary'}`}>
            {node.role}
          </span>
          <span className="flex items-center gap-1.5">
            <span className={`inline-block w-2 h-2 rounded-full ${statusDot[node.status] ?? 'bg-zinc-500'}`} />
            <span className="text-sm text-fg-secondary">{node.status}</span>
          </span>
        </div>
        <button
          onClick={onClose}
          className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors"
          data-testid="node-detail-close"
        >
          Close
        </button>
      </div>

      {/* Specs grid */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="bg-surface rounded-lg p-3" data-testid="node-platform">
          <div className="text-xs text-fg-muted">Platform</div>
          <div className="text-sm font-medium text-fg mt-1">{node.platform}</div>
        </div>
        <div className="bg-surface rounded-lg p-3" data-testid="node-arch">
          <div className="text-xs text-fg-muted">Architecture</div>
          <div className="text-sm font-medium text-fg mt-1">{node.arch}</div>
        </div>
        <div className="bg-surface rounded-lg p-3" data-testid="node-host">
          <div className="text-xs text-fg-muted">Host</div>
          <div className="text-sm font-medium text-fg mt-1 font-mono">{node.host}:{node.port}</div>
        </div>
        <div className="bg-surface rounded-lg p-3" data-testid="node-score">
          <div className="text-xs text-fg-muted">Capability Score</div>
          <div className="text-sm font-medium text-fg mt-1">{node.capability_score ?? '--'}</div>
        </div>
      </div>

      {/* Resources */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div className="bg-surface rounded-lg p-4" data-testid="node-cpu">
          <div className="text-xs text-fg-muted mb-1">CPU Cores</div>
          <div className="text-lg font-bold text-fg">{node.cpu_cores}</div>
        </div>
        <div className="bg-surface rounded-lg p-4" data-testid="node-ram">
          <div className="text-xs text-fg-muted mb-1">RAM</div>
          <div className="text-lg font-bold text-fg">{ramGb} GB</div>
          <div className="text-xs text-fg-muted">{node.ram_total_mb.toLocaleString()} MB</div>
        </div>
        <div className="bg-surface rounded-lg p-4" data-testid="node-storage">
          <div className="text-xs text-fg-muted mb-1">Storage</div>
          <div className="text-lg font-bold text-fg">{node.storage_total_gb} GB</div>
        </div>
      </div>

      {/* Uptime / last heartbeat */}
      <div className="bg-surface rounded-lg p-4" data-testid="node-uptime">
        <div className="text-xs text-fg-muted mb-1">Last Heartbeat</div>
        <div className="text-sm text-fg">
          {new Date(node.last_heartbeat).toLocaleString()}
          <span className="text-fg-muted ml-2">({relativeTime(node.last_heartbeat)})</span>
        </div>
      </div>

      {/* Heartbeat chart — CPU/RAM over time */}
      {heartbeats.length > 0 && (
        <div className="bg-surface rounded-lg p-4 space-y-3" data-testid="heartbeat-chart">
          <h4 className="text-sm font-medium text-fg-muted">Recent Heartbeats</h4>
          <div className="space-y-2">
            {heartbeats.slice(0, 10).map((hb, idx) => (
              <div key={idx} className="space-y-1 border-b border-border-subtle pb-2 last:border-0" data-testid={`heartbeat-${idx}`}>
                <div className="text-[10px] text-fg-muted font-mono">
                  {new Date(hb.timestamp).toLocaleTimeString()}
                </div>
                <MetricBar label="CPU" value={hb.cpu_usage_percent} max={100} unit="%" color="bg-soul" />
                <MetricBar label="RAM" value={hb.ram_used_percent} max={100} unit="%" color="bg-blue-400" />
                <div className="flex items-center gap-3 text-[10px] text-fg-muted">
                  <span>RAM avail: {hb.ram_available_mb.toLocaleString()} MB</span>
                  <span>Storage free: {hb.storage_free_gb} GB</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {heartbeats.length === 0 && (
        <div className="text-sm text-fg-muted py-4" data-testid="no-heartbeats">No heartbeat data available.</div>
      )}
    </div>
  );
}
