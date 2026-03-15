import type { MeshNode, Heartbeat } from '../../hooks/useMesh';

const roleColor: Record<string, string> = {
  hub: 'bg-soul-dim text-soul',
  agent: 'bg-blue-500/20 text-blue-400',
};

const statusColor: Record<string, string> = {
  online: 'text-emerald-400',
  offline: 'text-fg-muted',
  degraded: 'text-amber-400',
};

interface NodeDetailProps {
  node: MeshNode;
  heartbeats: Heartbeat[];
  onClose: () => void;
}

export function NodeDetail({ node, heartbeats, onClose }: NodeDetailProps) {
  const nodeHeartbeats = heartbeats.filter(h => h.node_id === node.id);

  return (
    <div className="space-y-4" data-testid={`node-detail-${node.id}`}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h3 className="text-base font-semibold text-fg">{node.name}</h3>
          <span className={`px-2 py-0.5 text-xs rounded-full ${roleColor[node.role] ?? 'bg-overlay text-fg-secondary'}`}>
            {node.role}
          </span>
          <span className={`text-sm ${statusColor[node.status] ?? 'text-fg-muted'}`}>
            {node.status}
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

      {/* Stats grid */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="bg-surface rounded-lg p-3">
          <div className="text-xs text-fg-muted">Platform</div>
          <div className="text-sm font-medium text-fg mt-1">{node.platform}</div>
        </div>
        <div className="bg-surface rounded-lg p-3">
          <div className="text-xs text-fg-muted">Architecture</div>
          <div className="text-sm font-medium text-fg mt-1">{node.arch}</div>
        </div>
        <div className="bg-surface rounded-lg p-3">
          <div className="text-xs text-fg-muted">CPU Cores</div>
          <div className="text-sm font-medium text-fg mt-1">{node.cpu_cores}</div>
        </div>
        <div className="bg-surface rounded-lg p-3">
          <div className="text-xs text-fg-muted">Capability Score</div>
          <div className="text-sm font-medium text-fg mt-1">{node.capability_score}</div>
        </div>
      </div>

      {/* Resources */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div className="bg-surface rounded-lg p-4">
          <div className="text-xs text-fg-muted mb-1">RAM</div>
          <div className="text-lg font-bold text-fg">{node.ram_total_gb} GB</div>
        </div>
        <div className="bg-surface rounded-lg p-4">
          <div className="text-xs text-fg-muted mb-1">Storage</div>
          <div className="text-lg font-bold text-fg">{node.storage_total_gb} GB</div>
        </div>
      </div>

      {/* Last heartbeat */}
      <div className="bg-surface rounded-lg p-4">
        <div className="text-xs text-fg-muted mb-1">Last Heartbeat</div>
        <div className="text-sm text-fg">{new Date(node.last_heartbeat).toLocaleString()}</div>
      </div>

      {/* Heartbeat history */}
      {nodeHeartbeats.length > 0 && (
        <div className="bg-surface rounded-lg p-4 space-y-2">
          <h4 className="text-sm font-medium text-fg-muted">Heartbeat History</h4>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-fg-muted border-b border-border-subtle">
                  <th className="text-left py-1.5 px-2 font-medium">Timestamp</th>
                  <th className="text-left py-1.5 px-2 font-medium">Status</th>
                  <th className="text-right py-1.5 px-2 font-medium">Latency (ms)</th>
                </tr>
              </thead>
              <tbody>
                {nodeHeartbeats.slice(0, 10).map((hb, idx) => (
                  <tr key={idx} className="border-b border-border-subtle">
                    <td className="py-1.5 px-2 text-fg-secondary">{new Date(hb.timestamp).toLocaleString()}</td>
                    <td className={`py-1.5 px-2 ${hb.status === 'ok' ? 'text-emerald-400' : 'text-amber-400'}`}>{hb.status}</td>
                    <td className="py-1.5 px-2 text-right text-fg-secondary font-mono">{hb.latency_ms}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
