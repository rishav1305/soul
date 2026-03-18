import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

type MeshTab = 'cluster' | 'nodes';

export interface MeshNode {
  id: string;
  name: string;
  host: string;
  port: number;
  role: string;
  platform: string;
  arch: string;
  cpu_cores: number;
  ram_total_mb: number;
  storage_total_gb: number;
  status: string;
  last_heartbeat: string;
  capability_score?: number;
}

export interface ClusterStatus {
  total_nodes: number;
  total_cpu: number;
  total_ram_mb: number;
  total_storage_gb: number;
  hub_id: string;
}

export interface Heartbeat {
  cpu_usage_percent: number;
  ram_available_mb: number;
  ram_used_percent: number;
  storage_free_gb: number;
  timestamp: string;
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
  fetchHeartbeats: (nodeId: string) => Promise<void>;
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
          const status = await api.get<ClusterStatus>('/api/mesh/status');
          setClusterStatus(status);
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

  const fetchHeartbeats = useCallback(async (nodeId: string) => {
    try {
      const data = await api.get<Heartbeat[]>(`/api/mesh/heartbeats?node_id=${nodeId}`);
      setHeartbeats(data ?? []);
    } catch (err: unknown) {
      reportError('useMesh.fetchHeartbeats', err);
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
    generateCode, linkNode, fetchHeartbeats,
  };
}
