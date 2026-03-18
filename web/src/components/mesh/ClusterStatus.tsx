import type { ClusterStatus as ClusterStatusType } from '../../hooks/useMesh';

interface ClusterStatusProps {
  status: ClusterStatusType;
}

function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="bg-surface rounded-lg p-4" data-testid={`mesh-stat-${label.toLowerCase().replace(/\s+/g, '-')}`}>
      <div className="text-xs text-fg-muted mb-1">{label}</div>
      <div className="text-2xl font-bold text-fg">{value}</div>
      {sub && <div className="text-xs text-fg-muted mt-1">{sub}</div>}
    </div>
  );
}

function healthLabel(status: ClusterStatusType): { text: string; color: string } {
  const offline = status.total_nodes > 0 ? status.total_nodes - Math.min(status.total_nodes, status.total_cpu > 0 ? status.total_nodes : 0) : 0;
  if (status.total_nodes === 0) return { text: 'No Nodes', color: 'text-fg-muted' };
  if (offline > 0) return { text: 'Degraded', color: 'text-amber-400' };
  return { text: 'Healthy', color: 'text-emerald-400' };
}

export function ClusterStatus({ status }: ClusterStatusProps) {
  const health = healthLabel(status);
  const ramGb = (status.total_ram_mb / 1024).toFixed(1);

  return (
    <div className="space-y-4" data-testid="cluster-status">
      {/* Hub identity + health */}
      <div className="bg-surface rounded-lg p-4">
        <div className="flex items-center gap-3">
          <span className="text-lg font-semibold text-fg">Cluster</span>
          <span className={`px-2 py-0.5 text-xs rounded-full font-medium ${health.color === 'text-emerald-400' ? 'bg-emerald-400/10 text-emerald-400' : health.color === 'text-amber-400' ? 'bg-amber-400/10 text-amber-400' : 'bg-zinc-500/10 text-fg-muted'}`} data-testid="cluster-health">
            {health.text}
          </span>
        </div>
        <div className="mt-2 text-sm text-fg-muted font-mono" data-testid="cluster-hub-id">
          Hub: {status.hub_id}
        </div>
      </div>

      {/* Aggregated resources */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <StatCard label="Total Nodes" value={status.total_nodes} />
        <StatCard label="Total CPU" value={status.total_cpu} sub="cores" />
        <StatCard label="Total RAM" value={`${ramGb} GB`} sub={`${status.total_ram_mb.toLocaleString()} MB`} />
        <StatCard label="Total Storage" value={`${status.total_storage_gb} GB`} />
      </div>
    </div>
  );
}
