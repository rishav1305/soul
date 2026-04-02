// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useBench } from './useBench';

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

function makeCategory(overrides: Record<string, unknown> = {}) {
  return { id: 'cat-1', name: 'Reasoning', prompt_count: 10, ...overrides };
}

function makeResult(overrides: Record<string, unknown> = {}) {
  return {
    id: 'res-1',
    model_name: 'claude-3',
    timestamp: '2026-03-30T10:00:00Z',
    accuracy: 0.85,
    latency_s: 1.2,
    cars_ram: 512,
    cars_size: 100,
    ...overrides,
  };
}

function makeResultDetail(overrides: Record<string, unknown> = {}) {
  return {
    ...makeResult(),
    tokens_per_sec: 50,
    peak_ram_mb: 1024,
    category_scores: [],
    prompt_details: [],
    ...overrides,
  };
}

describe('useBench', () => {
  beforeEach(() => {
    mockGet.mockReset();
    mockPost.mockReset();
    // Default: 'run' tab fetches categories on mount
    mockGet.mockResolvedValue([makeCategory()]);
  });
  afterEach(() => cleanup());

  it('fetches categories on mount (run tab)', async () => {
    const { result } = renderHook(() => useBench());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/bench/prompts');
    expect(result.current.categories).toEqual([makeCategory()]);
    expect(result.current.activeTab).toBe('run');
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useBench());
    expect(result.current.loading).toBe(true);
  });

  it('sets error on fetch failure', async () => {
    mockGet.mockRejectedValue(new Error('Server error'));
    const { result } = renderHook(() => useBench());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('Server error');
  });

  it('fetches results when switching to results tab', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockResolvedValue([makeResult()]);

    act(() => {
      result.current.setActiveTab('results');
    });

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/bench/results');
    expect(result.current.results).toEqual([makeResult()]);
    expect(result.current.activeTab).toBe('results');
  });

  it('fetches results when switching to compare tab', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockResolvedValue([makeResult()]);

    act(() => {
      result.current.setActiveTab('compare');
    });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(mockGet).toHaveBeenCalledWith('/api/bench/results');
  });

  it('fetchResultDetail sets selectedResult', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const detail = makeResultDetail();
    mockGet.mockResolvedValue(detail);

    await act(async () => {
      await result.current.fetchResultDetail('res-1');
    });

    expect(mockGet).toHaveBeenCalledWith('/api/bench/results/res-1');
    expect(result.current.selectedResult).toEqual(detail);
  });

  it('fetchResultDetail sets error on failure', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockRejectedValue(new Error('Not found'));

    await act(async () => {
      await result.current.fetchResultDetail('bad-id');
    });

    expect(result.current.error).toBe('Not found');
  });

  it('runBenchmark posts config and refreshes results', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(undefined);
    mockGet.mockResolvedValue([makeResult({ id: 'new-1' })]);

    const config = { endpoint: 'http://localhost:8080', categories: ['cat-1'], gpu: false, max_tokens: 1024 };
    await act(async () => {
      await result.current.runBenchmark(config);
    });

    expect(mockPost).toHaveBeenCalledWith('/api/bench/run', config);
    expect(mockGet).toHaveBeenCalledWith('/api/bench/results');
    expect(result.current.results).toEqual([makeResult({ id: 'new-1' })]);
  });

  it('runBenchmark sets error on failure', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockRejectedValue(new Error('Benchmark failed'));

    await act(async () => {
      await result.current.runBenchmark({ endpoint: 'x', categories: [], gpu: false, max_tokens: 100 });
    });

    expect(result.current.error).toBe('Benchmark failed');
  });

  it('runSmoke returns smoke results', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const smokeResults = [{ name: 'hello', passed: true, response_preview: 'Hello!' }];
    mockPost.mockResolvedValue(smokeResults);

    let res: unknown;
    await act(async () => {
      res = await result.current.runSmoke('http://localhost:8080');
    });

    expect(mockPost).toHaveBeenCalledWith('/api/bench/smoke', { endpoint: 'http://localhost:8080' });
    expect(res).toEqual(smokeResults);
  });

  it('runSmoke returns empty array on failure', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockRejectedValue(new Error('Smoke failed'));

    let res: unknown;
    await act(async () => {
      res = await result.current.runSmoke('bad');
    });

    expect(res).toEqual([]);
    expect(result.current.error).toBe('Smoke failed');
  });

  it('compare fetches and sets compareData', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const cmp = { result1: makeResultDetail({ id: 'a' }), result2: makeResultDetail({ id: 'b' }) };
    mockGet.mockResolvedValue(cmp);

    await act(async () => {
      await result.current.compare('a', 'b');
    });

    expect(mockGet).toHaveBeenCalledWith('/api/bench/compare?id1=a&id2=b');
    expect(result.current.compareData).toEqual(cmp);
  });

  it('compare sets error on failure', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockRejectedValue(new Error('Compare failed'));

    await act(async () => {
      await result.current.compare('a', 'b');
    });

    expect(result.current.error).toBe('Compare failed');
  });

  it('refresh re-fetches current tab data', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue([makeCategory({ id: 'cat-2' })]);

    act(() => {
      result.current.refresh();
    });

    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/bench/prompts'));
  });

  it('handles null response from API gracefully', async () => {
    mockGet.mockResolvedValue(null);
    const { result } = renderHook(() => useBench());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.categories).toEqual([]);
  });

  it('handles non-Error rejection in fetch', async () => {
    mockGet.mockRejectedValue('string error');
    const { result } = renderHook(() => useBench());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('string error');
  });

  it('handles non-Error rejection in runBenchmark', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockRejectedValue('run string error');
    await act(async () => {
      await result.current.runBenchmark({ endpoint: 'x', categories: [], gpu: false, max_tokens: 100 });
    });

    expect(result.current.error).toBe('run string error');
  });

  it('handles non-Error rejection in compare', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockRejectedValue('compare string error');
    await act(async () => {
      await result.current.compare('a', 'b');
    });

    expect(result.current.error).toBe('compare string error');
  });

  it('handles non-Error rejection in fetchResultDetail', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockRejectedValue('detail string error');
    await act(async () => {
      await result.current.fetchResultDetail('bad');
    });

    expect(result.current.error).toBe('detail string error');
  });

  it('handles non-Error rejection in runSmoke', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockRejectedValue('smoke string error');
    let res: unknown;
    await act(async () => {
      res = await result.current.runSmoke('bad');
    });

    expect(res).toEqual([]);
    expect(result.current.error).toBe('smoke string error');
  });

  it('error clears on successful refresh', async () => {
    mockGet.mockRejectedValue(new Error('initial fail'));
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.error).toBe('initial fail'));

    mockGet.mockResolvedValue([makeCategory()]);
    act(() => { result.current.refresh(); });
    await waitFor(() => expect(result.current.error).toBeNull());
  });

  it('runSmoke handles null response', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(null);
    let res: unknown;
    await act(async () => {
      res = await result.current.runSmoke('http://localhost');
    });
    expect(res).toEqual([]);
  });

  it('runBenchmark handles null results response', async () => {
    const { result } = renderHook(() => useBench());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(undefined);
    mockGet.mockResolvedValue(null);

    await act(async () => {
      await result.current.runBenchmark({ endpoint: 'x', categories: [], gpu: false, max_tokens: 100 });
    });
    expect(result.current.results).toEqual([]);
  });

  it('selectedResult is null initially', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useBench());
    expect(result.current.selectedResult).toBeNull();
  });

  it('compareData is null initially', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useBench());
    expect(result.current.compareData).toBeNull();
  });
});
