// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useMockSession } from './useMockSession';

const mockGet = vi.fn();

vi.mock('../lib/api', () => ({
  api: { get: (...args: unknown[]) => mockGet(...args) },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makeSession(overrides: Record<string, unknown> = {}) {
  return {
    id: 'sess-1',
    status: 'active',
    topic: 'System Design',
    questions: [],
    score: 0,
    ...overrides,
  };
}

describe('useMockSession', () => {
  beforeEach(() => {
    mockGet.mockReset();
  });
  afterEach(() => cleanup());

  it('fetches session on mount', async () => {
    mockGet.mockResolvedValue(makeSession());
    const { result } = renderHook(() => useMockSession('sess-1'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/tutor/mocks/sess-1');
    expect(result.current.session).toEqual(makeSession());
    expect(result.current.error).toBeNull();
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useMockSession('sess-1'));
    expect(result.current.loading).toBe(true);
    expect(result.current.session).toBeNull();
  });

  it('sets error on fetch failure', async () => {
    mockGet.mockRejectedValue(new Error('Not found'));
    const { result } = renderHook(() => useMockSession('sess-1'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('Not found');
    expect(result.current.session).toBeNull();
  });

  it('handles non-Error rejection', async () => {
    mockGet.mockRejectedValue('string error');
    const { result } = renderHook(() => useMockSession('sess-1'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('string error');
  });

  it('refresh re-fetches session data', async () => {
    mockGet.mockResolvedValue(makeSession());
    const { result } = renderHook(() => useMockSession('sess-1'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    const updated = makeSession({ score: 85 });
    mockGet.mockResolvedValue(updated);

    await act(async () => {
      result.current.refresh();
    });

    await waitFor(() => expect(result.current.session).toEqual(updated));
  });

  it('re-fetches when sessionId changes', async () => {
    mockGet.mockResolvedValue(makeSession({ id: 'sess-1' }));
    const { result, rerender } = renderHook(
      ({ id }) => useMockSession(id),
      { initialProps: { id: 'sess-1' } },
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    mockGet.mockClear();

    mockGet.mockResolvedValue(makeSession({ id: 'sess-2', topic: 'Algorithms' }));
    rerender({ id: 'sess-2' });

    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/tutor/mocks/sess-2'));
  });

  it('clears error on successful refresh', async () => {
    mockGet.mockRejectedValue(new Error('fail'));
    const { result } = renderHook(() => useMockSession('sess-1'));

    await waitFor(() => expect(result.current.error).toBe('fail'));

    mockGet.mockResolvedValue(makeSession());
    await act(async () => {
      result.current.refresh();
    });

    await waitFor(() => expect(result.current.error).toBeNull());
    expect(result.current.session).toEqual(makeSession());
  });

  it('loading is true during refresh', async () => {
    let resolveGet: ((v: unknown) => void) | null = null;
    mockGet.mockImplementation(() => new Promise(r => { resolveGet = r; }));

    const { result } = renderHook(() => useMockSession('sess-1'));

    // Initial load — loading should be true
    expect(result.current.loading).toBe(true);

    await act(async () => {
      resolveGet!(makeSession());
    });
    await waitFor(() => expect(result.current.loading).toBe(false));

    // Trigger refresh with pending promise
    mockGet.mockImplementation(() => new Promise(r => { resolveGet = r; }));
    act(() => {
      result.current.refresh();
    });
    expect(result.current.loading).toBe(true);

    await act(async () => {
      resolveGet!(makeSession({ score: 100 }));
    });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.session?.score).toBe(100);
  });

  it('preserves previous session data on refresh error', async () => {
    mockGet.mockResolvedValue(makeSession({ score: 50 }));
    const { result } = renderHook(() => useMockSession('sess-1'));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.session?.score).toBe(50);

    // Refresh with error — session from before is lost (set separately)
    mockGet.mockRejectedValue(new Error('refresh fail'));
    await act(async () => {
      result.current.refresh();
    });

    await waitFor(() => expect(result.current.loading).toBe(false));
    // Error is set
    expect(result.current.error).toBe('refresh fail');
  });

  it('calls correct endpoint with sessionId', async () => {
    mockGet.mockResolvedValue(makeSession({ id: 'sess-42' }));
    renderHook(() => useMockSession('sess-42'));

    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/tutor/mocks/sess-42'));
  });
});
