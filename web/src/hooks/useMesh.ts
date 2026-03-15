import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

type MeshTab = 'cluster' | 'nodes';

export interface MeshNode {
  id: string;
  name: string;
  role: 'hub' | 'agent';
  status: 'online' | 'offline' | 'degraded';
  platform: string;
  arch: string;
  cpu_cores: number;
  ram_total_gb: number;
  storage_total_gb: number;
  capability_score: number;
  last_heartbeat: string;
}

export interface ClusterStatus {
  name: string;
  role: string;
  total_cpu_cores: number;
  total_ram_gb: number;
  total_storage_gb: number;
  online_nodes: number;
  total_nodes: number;
}

export interface Heartbeat {
  node_id: string;
  node_name: string;
  timestamp: string;
  status: string;
  latency_ms: number;
}

interface LinkResponse {
  code?: string;
  message?: string;
}

interface UseMeshReturn {
  clusterStatus: ClusterStatus | null;
  nodes: MeshNode[];
  selectedNode: MeshNode | null;
  setSelectedNode: (node: MeshNode | null) => void;
  heartbeats: Heartbeat[];
  linkCode: string | null;
  loading: boolean;
  error: string | null;
  activeTab: MeshTab;
  setActiveTab: (tab: MeshTab) => void;
  refresh: () => void;
  generateCode: () => Promise<void>;
  linkNode: (code: string) => Promise<void>;
}

export function useMesh(): UseMeshReturn {
  const [clusterStatus, setClusterStatus] = useState<ClusterStatus | null>(null);
  const [nodes, setNodes] = useState<MeshNode[]>([]);
  const [selectedNode, setSelectedNode] = useState<MeshNode | null>(null);
  const [heartbeats, setHeartbeats] = useState<Heartbeat[]>([]);
  const [linkCode, setLinkCode] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<MeshTab>('cluster');

  const fetchData = useCallback(async (tab: MeshTab) => {
    setLoading(true);
    setError(null);
    try {
      switch (tab) {
        case 'cluster': {
          const [status, hb] = await Promise.all([
            api.get<ClusterStatus>('/api/mesh/status'),
            api.get<Heartbeat[]>('/api/mesh/heartbeats'),
          ]);
          setClusterStatus(status);
          setHeartbeats(hb ?? []);
          break;
        }
        case 'nodes': {
          const data = await api.get<MeshNode[]>('/api/mesh/nodes');
          setNodes(data ?? []);
          break;
        }
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useMesh.fetch', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  const handleSetTab = useCallback((tab: MeshTab) => {
    setActiveTab(tab);
    reportUsage('mesh.tab', { tab });
  }, []);

  const refresh = useCallback(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  const generateCode = useCallback(async () => {
    try {
      const res = await api.post<LinkResponse>('/api/mesh/link', { action: 'generate' });
      setLinkCode(res?.code ?? null);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useMesh.generateCode', err);
      setError(message);
    }
  }, []);

  const linkNode = useCallback(async (code: string) => {
    try {
      await api.post<LinkResponse>('/api/mesh/link', { code });
      setLinkCode(null);
      fetchData(activeTab);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useMesh.linkNode', err);
      setError(message);
    }
  }, [activeTab, fetchData]);

  return {
    clusterStatus, nodes, selectedNode, setSelectedNode,
    heartbeats, linkCode, loading, error,
    activeTab, setActiveTab: handleSetTab, refresh,
    generateCode, linkNode,
  };
}
