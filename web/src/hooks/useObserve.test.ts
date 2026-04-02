// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useObserve } from './useObserve';

const mockGet = vi.fn();

vi.mock('../lib/api', () => ({
  api: { get: (...args: unknown[]) => mockGet(...args) },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makePillar(overrides: Record<string, unknown> = {}) {
  return { name: 'performant', score: 0.9, metrics: [], ...overrides };
}

function makeOverview(overrides: Record<string, unknown> = {}) {
  return { total_events: 100, error_rate: 0.02, uptime_pct: 99.9, ...overrides };
}

function makeTail(overrides: Record<string, unknown> = {}) {
  return { events: [{ type: 'info', message: 'ok', timestamp: '2026-03-30T10:00:00Z' }], ...overrides };
}

describe('useObserve', () => {
  beforeEach(() => {
    mockGet.mockReset();
    // Default: overview tab fetches pillars + overview on mount
    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve({ pillars: [makePillar()] });
      if (path === '/api/observe/overview') return Promise.resolve(makeOverview());
      if (path.startsWith('/api/observe/tail')) return Promise.resolve(makeTail());
      return Promise.resolve(null);
    });
  });
  afterEach(() => cleanup());

  it('fetches pillars and overview on mount', async () => {
    const { result } = renderHook(() => useObserve());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.pillars).toEqual([makePillar()]);
    expect(result.current.overview).toEqual(makeOverview());
    expect(result.current.activeTab).toBe('overview');
    expect(result.current.error).toBeNull();
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useObserve());
    expect(result.current.loading).toBe(true);
  });

  it('sets error on fetch failure', async () => {
    mockGet.mockRejectedValue(new Error('Server down'));
    const { result } = renderHook(() => useObserve());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Server down');
  });

  it('fetches tail data when switching to tail tab', async () => {
    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      result.current.setActiveTab('tail');
    });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.tail).toEqual(makeTail());
    expect(result.current.activeTab).toBe('tail');
  });

  it('fetches only pillars for pillar tabs', async () => {
    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve({ pillars: [makePillar({ name: 'secure' })] });
      return Promise.resolve(null);
    });

    act(() => {
      result.current.setActiveTab('secure');
    });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.pillars).toEqual([makePillar({ name: 'secure' })]);
  });

  it('passes product filter to pillars endpoint', async () => {
    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve({ pillars: [] });
      if (path === '/api/observe/overview') return Promise.resolve(makeOverview());
      return Promise.resolve(null);
    });

    act(() => {
      result.current.setProduct('chat');
    });

    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/observe/pillars?product=chat'));
  });

  it('handles null pillars response', async () => {
    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve(null);
      if (path === '/api/observe/overview') return Promise.resolve(makeOverview());
      return Promise.resolve(null);
    });

    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.pillars).toEqual([]);
  });

  it('refresh re-fetches current tab', async () => {
    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve({ pillars: [makePillar()] });
      if (path === '/api/observe/overview') return Promise.resolve(makeOverview({ total_events: 200 }));
      return Promise.resolve(null);
    });

    act(() => {
      result.current.refresh();
    });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.overview).toEqual(makeOverview({ total_events: 200 }));
  });

  it('product defaults to empty string', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useObserve());
    expect(result.current.product).toBe('');
  });

  it('handles non-Error rejection', async () => {
    mockGet.mockRejectedValue('string error');
    const { result } = renderHook(() => useObserve());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('string error');
  });

  it('error clears on successful refresh', async () => {
    mockGet.mockRejectedValue(new Error('initial fail'));
    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.error).toBe('initial fail'));

    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve({ pillars: [makePillar()] });
      if (path === '/api/observe/overview') return Promise.resolve(makeOverview());
      return Promise.resolve(null);
    });
    act(() => { result.current.refresh(); });
    await waitFor(() => expect(result.current.error).toBeNull());
  });

  it('overview is null initially', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useObserve());
    expect(result.current.overview).toBeNull();
  });

  it('tail is null initially', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useObserve());
    expect(result.current.tail).toBeNull();
  });

  it('pillar tabs do not fetch overview or tail', async () => {
    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve({ pillars: [makePillar({ name: 'robust' })] });
      return Promise.resolve(null);
    });

    act(() => { result.current.setActiveTab('robust'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    // Should have only called pillars endpoint, not overview or tail
    const calls = mockGet.mock.calls.map((c: unknown[]) => c[0] as string);
    expect(calls.every((c: string) => c.startsWith('/api/observe/pillars'))).toBe(true);
  });

  it('setProduct triggers refetch with product filter', async () => {
    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve({ pillars: [] });
      if (path === '/api/observe/overview') return Promise.resolve(makeOverview());
      return Promise.resolve(null);
    });

    act(() => { result.current.setProduct('scout'); });
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/observe/pillars?product=scout'));
    expect(result.current.product).toBe('scout');
  });

  it('handles null pillars response inside nested object', async () => {
    mockGet.mockImplementation((path: string) => {
      if (path.startsWith('/api/observe/pillars')) return Promise.resolve({ pillars: null });
      if (path === '/api/observe/overview') return Promise.resolve(makeOverview());
      return Promise.resolve(null);
    });

    const { result } = renderHook(() => useObserve());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.pillars).toEqual([]);
  });
});
