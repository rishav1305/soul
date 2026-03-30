// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useMesh } from './useMesh';

const mockGet = vi.fn();
const mockPost = vi.fn();

vi.mock('../lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockGet(...args),
    post: (...args: unknown[]) => mockPost(...args),
  },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makeCluster(overrides: Record<string, unknown> = {}) {
  return {
    total_nodes: 2,
    total_cpu: 12,
    total_ram_mb: 16384,
    total_storage_gb: 500,
    hub_id: 'node-1',
    ...overrides,
  };
}

function makeNode(overrides: Record<string, unknown> = {}) {
  return {
    id: 'node-1',
    name: 'titan-pc',
    host: '192.168.0.196',
    port: 3024,
    role: 'hub',
    platform: 'linux',
    arch: 'x86_64',
    cpu_cores: 8,
    ram_total_mb: 16384,
    storage_total_gb: 500,
    status: 'active',
    last_heartbeat: '2026-03-30T16:00:00Z',
    ...overrides,
  };
}

describe('useMesh', () => {
  beforeEach(() => {
    mockGet.mockReset();
    mockPost.mockReset();
    mockGet.mockResolvedValue(makeCluster());
  });
  afterEach(() => cleanup());

  it('fetches cluster status on mount', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/mesh/status');
    expect(result.current.clusterStatus).toEqual(makeCluster());
    expect(result.current.activeTab).toBe('cluster');
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useMesh());
    expect(result.current.loading).toBe(true);
  });

  it('sets error on failure', async () => {
    mockGet.mockRejectedValue(new Error('Mesh down'));
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Mesh down');
  });

  it('fetches nodes when switching to nodes tab', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const nodes = [makeNode()];
    mockGet.mockResolvedValue(nodes);

    act(() => { result.current.setActiveTab('nodes'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/mesh/nodes');
    expect(result.current.nodes).toEqual(nodes);
  });

  it('handles null nodes response', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockResolvedValue(null);
    act(() => { result.current.setActiveTab('nodes'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.nodes).toEqual([]);
  });

  it('fetchHeartbeats fetches node heartbeats', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const hb = [{ cpu_usage_percent: 50, ram_available_mb: 8000, ram_used_percent: 50, storage_free_gb: 250, timestamp: '2026-03-30T16:00:00Z' }];
    mockGet.mockResolvedValue(hb);

    await act(async () => {
      await result.current.fetchHeartbeats('node-1');
    });

    expect(mockGet).toHaveBeenCalledWith('/api/mesh/heartbeats?node_id=node-1');
    expect(result.current.heartbeats).toEqual(hb);
  });

  it('generateCode posts to link endpoint', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue({ code: 'ABC123' });

    await act(async () => {
      await result.current.generateCode();
    });

    expect(mockPost).toHaveBeenCalledWith('/api/mesh/link', { action: 'generate' });
    expect(result.current.linkCode).toBe('ABC123');
  });

  it('generateCode sets error on failure', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockRejectedValue(new Error('Generate failed'));

    await act(async () => {
      await result.current.generateCode();
    });

    expect(result.current.error).toBe('Generate failed');
  });

  it('linkNode posts code and refreshes', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue({ message: 'linked' });
    mockGet.mockResolvedValue(makeCluster({ total_nodes: 3 }));

    await act(async () => {
      await result.current.linkNode('ABC123');
    });

    expect(mockPost).toHaveBeenCalledWith('/api/mesh/link', { code: 'ABC123' });
    expect(result.current.linkCode).toBeNull();
  });

  it('linkNode sets error on failure', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockRejectedValue(new Error('Invalid code'));

    await act(async () => {
      await result.current.linkNode('BAD');
    });

    expect(result.current.error).toBe('Invalid code');
  });

  it('refresh re-fetches current tab', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue(makeCluster({ total_nodes: 5 }));

    act(() => { result.current.refresh(); });
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/mesh/status'));
  });

  it('setSelectedNode updates state', async () => {
    const { result } = renderHook(() => useMesh());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const node = makeNode();
    act(() => { result.current.setSelectedNode(node as any); });
    expect(result.current.selectedNode).toEqual(node);

    act(() => { result.current.setSelectedNode(null); });
    expect(result.current.selectedNode).toBeNull();
  });
});
