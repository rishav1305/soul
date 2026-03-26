import { useEffect, useCallback } from 'react';
import { useMesh } from '../hooks/useMesh';
import type { MeshNode } from '../hooks/useMesh';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import { ClusterStatus } from '../components/mesh/ClusterStatus';
import { NodeList } from '../components/mesh/NodeList';
import { NodeDetail } from '../components/mesh/NodeDetail';
import { LinkingPanel } from '../components/mesh/LinkingPanel';

export function MeshPage() {
  usePerformance('MeshPage');
  const {
    clusterStatus, nodes, selectedNode, setSelectedNode,
    heartbeats, linkCode, loading, error,
    activeTab, setActiveTab, refresh,
    generateCode, linkNode, fetchHeartbeats,
  } = useMesh();

  useEffect(() => { reportUsage('page.view', { page: 'mesh' }); }, []);

  const tabs = ['cluster', 'nodes'] as const;

  const handleSelectNode = useCallback((node: MeshNode) => {
    setSelectedNode(node);
    fetchHeartbeats(node.id);
  }, [setSelectedNode, fetchHeartbeats]);

  return (
    <div data-testid="mesh-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-fg">Mesh</h2>
        <button onClick={refresh} className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors" data-testid="mesh-refresh">Refresh</button>
      </div>

      {/* Tab nav */}
      <nav className="tab-scroll flex gap-1 border-b border-border-subtle pb-px" data-testid="mesh-tabs" role="tablist" aria-label="Mesh sections">
        {tabs.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            role="tab"
            aria-selected={activeTab === tab}
            aria-controls={`mesh-panel-${tab}`}
            className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize ${activeTab === tab ? 'bg-surface text-fg' : 'text-fg-muted hover:text-zinc-200'}`}
            data-testid={`tab-${tab}`}>
            {tab}
          </button>
        ))}
      </nav>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" role="alert" data-testid="mesh-error">{error}</div>}

      {/* Tab content */}
      {activeTab === 'cluster' && clusterStatus && (
        <div className="space-y-4">
          <ClusterStatus status={clusterStatus} />
          <LinkingPanel linkCode={linkCode} onGenerateCode={generateCode} onLinkNode={linkNode} />
        </div>
      )}

      {activeTab === 'nodes' && (
        <div className="space-y-4">
          {selectedNode ? (
            <NodeDetail node={selectedNode} heartbeats={heartbeats} onClose={() => setSelectedNode(null)} />
          ) : (
            <NodeList nodes={nodes} selectedId={null} onSelect={handleSelectNode} />
          )}
        </div>
      )}

      {loading && !clusterStatus && nodes.length === 0 && (
        <div className="text-center py-8 text-fg-muted" role="status" aria-live="polite">Loading...</div>
      )}
    </div>
  );
}
