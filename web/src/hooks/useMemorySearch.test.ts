// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useMemorySearch } from './useMemorySearch';

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

const mockFetch = vi.fn();

describe('useMemorySearch', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it('returns initial state', () => {
    const { result } = renderHook(() => useMemorySearch());
    expect(result.current.results).toEqual([]);
    expect(result.current.health).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(result.current.healthLoading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.healthError).toBeNull();
  });

  it('search calls /memory/query with correct body', async () => {
    const mockResponse = {
      results: [{ doc_id: 'test', content: 'data', metadata: {}, score: 0.9 }],
      query: 'test query',
      collection: 'soul_agent_memory',
      total_in_collection: 109,
      latency_ms: 42.5,
    };
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.search({
        collection: 'soul_agent_memory',
        query: 'test query',
        top_k: 5,
      });
    });

    expect(mockFetch).toHaveBeenCalledWith(
      'http://192.168.0.196:3030/memory/query',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          collection: 'soul_agent_memory',
          query: 'test query',
          top_k: 5,
        }),
      }),
    );
    expect(result.current.results).toEqual(mockResponse.results);
    expect(result.current.loading).toBe(false);
  });

  it('search sets error on non-ok response', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve({ detail: 'ChromaDB unavailable' }),
    });

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.search({
        collection: 'soul_agent_memory',
        query: 'test',
        top_k: 5,
      });
    });

    expect(result.current.error).toBe('ChromaDB unavailable');
    expect(result.current.results).toEqual([]);
  });

  it('search sets error on network failure', async () => {
    mockFetch.mockRejectedValue(new TypeError('Failed to fetch'));

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.search({
        collection: 'soul_agent_memory',
        query: 'test',
        top_k: 5,
      });
    });

    expect(result.current.error).toBe('Failed to fetch');
  });

  it('search returns empty array on error', async () => {
    mockFetch.mockRejectedValue(new Error('timeout'));

    const { result } = renderHook(() => useMemorySearch());

    let returnValue: unknown[];
    await act(async () => {
      returnValue = await result.current.search({
        collection: 'soul_agent_memory',
        query: 'test',
        top_k: 5,
      });
    });

    expect(returnValue!).toEqual([]);
  });

  it('search clears previous error on new search', async () => {
    mockFetch.mockRejectedValueOnce(new Error('first fail'));

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.search({ collection: 'soul_agent_memory', query: 'fail', top_k: 5 });
    });
    expect(result.current.error).toBe('first fail');

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ results: [], query: 'succeed', collection: 'soul_agent_memory', total_in_collection: 0, latency_ms: 1 }),
    });

    await act(async () => {
      await result.current.search({ collection: 'soul_agent_memory', query: 'succeed', top_k: 5 });
    });
    expect(result.current.error).toBeNull();
  });

  it('search includes agent_filter and type_filter when provided', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ results: [], query: 'test', collection: 'soul_agent_memory', total_in_collection: 0, latency_ms: 1 }),
    });

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.search({
        collection: 'soul_agent_memory',
        query: 'test',
        agent_filter: 'shuri',
        type_filter: 'project',
        top_k: 3,
      });
    });

    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.agent_filter).toBe('shuri');
    expect(body.type_filter).toBe('project');
    expect(body.top_k).toBe(3);
  });

  it('fetchHealth calls /memory/health', async () => {
    const healthData = {
      chromadb: 'up',
      collections: { soul_agent_memory: 109, soul_shared_kb: 28 },
      last_index: '2026-03-30T16:00:00',
    };
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(healthData),
    });

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.fetchHealth();
    });

    expect(mockFetch).toHaveBeenCalledWith('http://192.168.0.196:3030/memory/health');
    expect(result.current.health).toEqual(healthData);
    expect(result.current.healthLoading).toBe(false);
  });

  it('fetchHealth sets healthError on failure', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
    });

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.fetchHealth();
    });

    expect(result.current.healthError).toBe('Health check failed: HTTP 503');
    expect(result.current.health).toBeNull();
  });

  it('fetchHealth sets healthError on network failure', async () => {
    mockFetch.mockRejectedValue(new TypeError('Network error'));

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.fetchHealth();
    });

    expect(result.current.healthError).toBe('Network error');
  });

  it('search and health loading are independent', async () => {
    mockFetch.mockImplementation(() => new Promise(() => {})); // never resolves

    const { result } = renderHook(() => useMemorySearch());

    // Start search
    act(() => {
      result.current.search({ collection: 'soul_agent_memory', query: 'test', top_k: 5 });
    });

    await waitFor(() => expect(result.current.loading).toBe(true));
    expect(result.current.healthLoading).toBe(false);
  });

  it('handles json parse failure on error response', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.reject(new Error('Invalid JSON')),
    });

    const { result } = renderHook(() => useMemorySearch());

    await act(async () => {
      await result.current.search({ collection: 'soul_agent_memory', query: 'test', top_k: 5 });
    });

    expect(result.current.error).toBe('Internal Server Error');
  });
});
