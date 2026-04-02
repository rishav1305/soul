// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useProjectDetail } from './useProjectDetail';

const mockGet = vi.fn();
const mockPost = vi.fn();
const mockPatch = vi.fn();
const mockAuthFetch = vi.fn();

vi.mock('../lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockGet(...args),
    post: (...args: unknown[]) => mockPost(...args),
    patch: (...args: unknown[]) => mockPatch(...args),
  },
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makeProject(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: 'Test Project',
    description: 'A test project',
    status: 'active',
    milestones: [],
    metrics: [],
    readiness: {},
    syncs: [],
    ...overrides,
  };
}

describe('useProjectDetail', () => {
  beforeEach(() => {
    mockGet.mockReset();
    mockPost.mockReset();
    mockPatch.mockReset();
    mockAuthFetch.mockReset();
    // Default: api.get returns project, authFetch returns guide
    mockGet.mockResolvedValue(makeProject());
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ content: 'Guide text' }),
    });
  });
  afterEach(() => cleanup());

  it('fetches project and guide on mount', async () => {
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/projects/1');
    expect(mockAuthFetch).toHaveBeenCalledWith('/api/projects/1/guide');
    expect(result.current.project).toEqual(makeProject());
    expect(result.current.guide).toBe('Guide text');
    expect(result.current.error).toBeNull();
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {})); // never resolves
    const { result } = renderHook(() => useProjectDetail(1));
    expect(result.current.loading).toBe(true);
    expect(result.current.project).toBeNull();
  });

  it('sets error on fetch failure', async () => {
    mockGet.mockRejectedValue(new Error('Not found'));
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('Not found');
    expect(result.current.project).toBeNull();
  });

  it('handles guide fetch failure gracefully', async () => {
    mockAuthFetch.mockResolvedValue({ ok: false });
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.project).toEqual(makeProject());
    expect(result.current.guide).toBe('');
  });

  it('handles guide fetch exception gracefully', async () => {
    mockAuthFetch.mockRejectedValue(new Error('network'));
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.project).toEqual(makeProject());
    expect(result.current.guide).toBe('');
  });

  it('updateProject calls api.patch and refetches', async () => {
    mockPatch.mockResolvedValue(undefined);
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));
    mockGet.mockClear();

    await act(async () => {
      await result.current.updateProject({ name: 'Updated' });
    });

    expect(mockPatch).toHaveBeenCalledWith('/api/projects/1', { name: 'Updated' });
    // Should re-fetch after update
    expect(mockGet).toHaveBeenCalledWith('/api/projects/1');
  });

  it('updateProject throws on failure', async () => {
    mockPatch.mockRejectedValue(new Error('Update failed'));
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    await expect(
      act(async () => {
        await result.current.updateProject({ name: 'x' });
      }),
    ).rejects.toThrow('Update failed');
  });

  it('updateMilestone calls api.patch with milestone path', async () => {
    mockPatch.mockResolvedValue(undefined);
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.updateMilestone(5, { completed: true });
    });

    expect(mockPatch).toHaveBeenCalledWith('/api/projects/1/milestones/5', { completed: true });
  });

  it('recordMetric calls api.post with metric data', async () => {
    mockPost.mockResolvedValue(undefined);
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.recordMetric('coverage', '85', 'percent');
    });

    expect(mockPost).toHaveBeenCalledWith('/api/projects/1/metrics', { name: 'coverage', value: '85', unit: 'percent' });
  });

  it('updateReadiness calls api.post with readiness fields', async () => {
    mockPost.mockResolvedValue(undefined);
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.updateReadiness({ tests_passing: true });
    });

    expect(mockPost).toHaveBeenCalledWith('/api/projects/1/readiness', { tests_passing: true });
  });

  it('syncPlatform calls api.post with platform data', async () => {
    mockPost.mockResolvedValue(undefined);
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.syncPlatform('github', true);
    });

    expect(mockPost).toHaveBeenCalledWith('/api/projects/1/syncs', { platform: 'github', synced: true });
  });

  it('refresh triggers re-fetch', async () => {
    const { result } = renderHook(() => useProjectDetail(1));

    await waitFor(() => expect(result.current.loading).toBe(false));
    mockGet.mockClear();

    act(() => {
      result.current.refresh();
    });

    expect(mockGet).toHaveBeenCalledWith('/api/projects/1');
  });

  it('re-fetches when projectId changes', async () => {
    const { result, rerender } = renderHook(
      ({ id }) => useProjectDetail(id),
      { initialProps: { id: 1 } },
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    mockGet.mockClear();
    mockAuthFetch.mockClear();

    rerender({ id: 2 });

    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/projects/2'));
    expect(mockAuthFetch).toHaveBeenCalledWith('/api/projects/2/guide');
  });

  it('handles non-Error rejection in fetch', async () => {
    mockGet.mockRejectedValue('raw string error');
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('raw string error');
  });

  it('error clears on successful refresh', async () => {
    mockGet.mockRejectedValue(new Error('first failure'));
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.error).toBe('first failure'));

    mockGet.mockResolvedValue(makeProject());
    act(() => { result.current.refresh(); });
    await waitFor(() => expect(result.current.error).toBeNull());
  });

  it('guide is empty string when response has no content field', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({}), // no content field
    });
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.guide).toBe('');
  });

  it('updateMilestone throws on failure', async () => {
    mockPatch.mockRejectedValue(new Error('Milestone update failed'));
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await expect(
      act(async () => { await result.current.updateMilestone(5, { completed: false }); }),
    ).rejects.toThrow('Milestone update failed');
  });

  it('recordMetric throws on failure', async () => {
    mockPost.mockRejectedValue(new Error('Metric post failed'));
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await expect(
      act(async () => { await result.current.recordMetric('cov', '80', 'pct'); }),
    ).rejects.toThrow('Metric post failed');
  });

  it('updateReadiness throws on failure', async () => {
    mockPost.mockRejectedValue(new Error('Readiness failed'));
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await expect(
      act(async () => { await result.current.updateReadiness({ tests_passing: false }); }),
    ).rejects.toThrow('Readiness failed');
  });

  it('syncPlatform throws on failure', async () => {
    mockPost.mockRejectedValue(new Error('Sync failed'));
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await expect(
      act(async () => { await result.current.syncPlatform('gitlab', false); }),
    ).rejects.toThrow('Sync failed');
  });

  it('updateMilestone refetches project after success', async () => {
    mockPatch.mockResolvedValue(undefined);
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.loading).toBe(false));
    mockGet.mockClear();

    await act(async () => { await result.current.updateMilestone(3, { title: 'v2' }); });
    expect(mockGet).toHaveBeenCalledWith('/api/projects/1');
  });

  it('recordMetric refetches project after success', async () => {
    mockPost.mockResolvedValue(undefined);
    const { result } = renderHook(() => useProjectDetail(1));
    await waitFor(() => expect(result.current.loading).toBe(false));
    mockGet.mockClear();

    await act(async () => { await result.current.recordMetric('uptime', '99.9', 'pct'); });
    expect(mockGet).toHaveBeenCalledWith('/api/projects/1');
  });
});
