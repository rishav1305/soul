import type { ClusterStatus as ClusterStatusType } from '../../hooks/useMesh';

interface ClusterStatusProps {
  status: ClusterStatusType;
}

function ResourceBar({ label, value, total, unit }: { label: string; value: number; total: number; unit: string }) {
  const percent = total > 0 ? Math.round((value / total) * 100) : 0;
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-xs">
        <span className="text-fg-muted">{label}</span>
        <span className="text-fg-secondary">{value} / {total} {unit}</span>
      </div>
      <div className="w-full h-1.5 bg-elevated rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${percent > 80 ? 'bg-red-500' : percent > 60 ? 'bg-amber-500' : 'bg-emerald-500'}`}
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  );
}

export function ClusterStatus({ status }: ClusterStatusProps) {
  return (
    <div className="space-y-4" data-testid="cluster-status">
      {/* Hub identity */}
      <div className="bg-surface rounded-lg p-4">
        <div className="flex items-center gap-3">
          <span className="text-lg font-semibold text-fg">{status.name}</span>
          <span className="px-2 py-0.5 text-xs rounded-full bg-soul-dim text-soul">{status.role}</span>
        </div>
        <div className="mt-2 text-sm text-fg-muted">
          {status.online_nodes} / {status.total_nodes} nodes online
        </div>
      </div>

      {/* Aggregated resources */}
      <div className="bg-surface rounded-lg p-4 space-y-3">
        <h3 className="text-sm font-medium text-fg-muted">Cluster Resources</h3>
        <ResourceBar label="CPU Cores" value={status.online_nodes} total={status.total_cpu_cores} unit="cores" />
        <ResourceBar label="RAM" value={Math.round(status.total_ram_gb * 0.7)} total={status.total_ram_gb} unit="GB" />
        <ResourceBar label="Storage" value={Math.round(status.total_storage_gb * 0.4)} total={status.total_storage_gb} unit="GB" />
      </div>

      {/* Node count card */}
      <div className="grid grid-cols-2 gap-3">
        <div className="bg-surface rounded-lg p-4 text-center">
          <div className="text-2xl font-bold text-emerald-400">{status.online_nodes}</div>
          <div className="text-xs text-fg-muted mt-1">Online</div>
        </div>
        <div className="bg-surface rounded-lg p-4 text-center">
          <div className="text-2xl font-bold text-fg-secondary">{status.total_nodes - status.online_nodes}</div>
          <div className="text-xs text-fg-muted mt-1">Offline</div>
        </div>
      </div>
    </div>
  );
}
