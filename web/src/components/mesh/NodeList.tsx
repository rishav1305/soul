import type { MeshNode } from '../../hooks/useMesh';

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

interface NodeListProps {
  nodes: MeshNode[];
  selectedId: string | null;
  onSelect: (node: MeshNode) => void;
}

export function NodeList({ nodes, selectedId, onSelect }: NodeListProps) {
  if (nodes.length === 0) {
    return <div className="text-sm text-fg-muted py-4" data-testid="node-list-empty">No nodes found.</div>;
  }

  return (
    <div className="overflow-x-auto" data-testid="node-list">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-xs text-fg-muted border-b border-border-subtle">
            <th className="text-left py-2 px-3 font-medium">Name</th>
            <th className="text-left py-2 px-3 font-medium">Role</th>
            <th className="text-left py-2 px-3 font-medium">Status</th>
            <th className="text-left py-2 px-3 font-medium hidden sm:table-cell">Platform</th>
            <th className="text-left py-2 px-3 font-medium hidden md:table-cell">Last Heartbeat</th>
            <th className="text-right py-2 px-3 font-medium">Score</th>
          </tr>
        </thead>
        <tbody>
          {nodes.map(node => (
            <tr
              key={node.id}
              onClick={() => onSelect(node)}
              className={`border-b border-border-subtle cursor-pointer transition-colors ${selectedId === node.id ? 'bg-elevated' : 'hover:bg-surface'}`}
              data-testid={`node-row-${node.id}`}
            >
              <td className="py-2 px-3 text-fg font-medium">{node.name}</td>
              <td className="py-2 px-3">
                <span className={`px-2 py-0.5 text-xs rounded-full ${roleBadge[node.role] ?? 'bg-overlay text-fg-secondary'}`}>
                  {node.role}
                </span>
              </td>
              <td className="py-2 px-3">
                <span className="flex items-center gap-1.5">
                  <span className={`inline-block w-2 h-2 rounded-full ${statusDot[node.status] ?? 'bg-zinc-500'}`} />
                  <span className="text-fg-secondary">{node.status}</span>
                </span>
              </td>
              <td className="py-2 px-3 text-fg-muted hidden sm:table-cell">
                {node.platform}/{node.arch}
              </td>
              <td className="py-2 px-3 text-fg-muted hidden md:table-cell">
                {relativeTime(node.last_heartbeat)}
              </td>
              <td className="py-2 px-3 text-right">
                {node.capability_score != null ? (
                  <div className="flex items-center justify-end gap-2">
                    <div className="w-16 h-1.5 bg-elevated rounded-full overflow-hidden">
                      <div
                        className="h-full rounded-full bg-soul"
                        style={{ width: `${Math.min(node.capability_score, 100)}%` }}
                      />
                    </div>
                    <span className="text-fg-secondary font-mono text-xs">{node.capability_score}</span>
                  </div>
                ) : (
                  <span className="text-fg-muted text-xs">--</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
