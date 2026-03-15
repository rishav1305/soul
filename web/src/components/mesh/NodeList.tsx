import type { MeshNode } from '../../hooks/useMesh';

const roleColor: Record<string, string> = {
  hub: 'bg-soul-dim text-soul',
  agent: 'bg-blue-500/20 text-blue-400',
};

const statusColor: Record<string, string> = {
  online: 'text-emerald-400',
  offline: 'text-fg-muted',
  degraded: 'text-amber-400',
};

interface NodeListProps {
  nodes: MeshNode[];
  selectedId: string | null;
  onSelect: (node: MeshNode) => void;
}

export function NodeList({ nodes, selectedId, onSelect }: NodeListProps) {
  if (nodes.length === 0) {
    return <div className="text-sm text-fg-muted py-4">No nodes found.</div>;
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
                <span className={`px-2 py-0.5 text-xs rounded-full ${roleColor[node.role] ?? 'bg-overlay text-fg-secondary'}`}>
                  {node.role}
                </span>
              </td>
              <td className={`py-2 px-3 ${statusColor[node.status] ?? 'text-fg-muted'}`}>
                {node.status}
              </td>
              <td className="py-2 px-3 text-fg-muted hidden sm:table-cell">
                {node.platform}/{node.arch}
              </td>
              <td className="py-2 px-3 text-fg-muted hidden md:table-cell">
                {new Date(node.last_heartbeat).toLocaleString()}
              </td>
              <td className="py-2 px-3 text-right text-fg-secondary font-mono">
                {node.capability_score}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
