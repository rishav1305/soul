// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useProjects } from './useProjects';

const mockGet = vi.fn();

vi.mock('../lib/api', () => ({
  api: { get: (...args: unknown[]) => mockGet(...args) },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makeDashboard(overrides: Record<string, unknown> = {}) {
  return { total_projects: 5, active: 3, completed: 2, ...overrides };
}

describe('useProjects', () => {
  beforeEach(() => {
    mockGet.mockReset();
    mockGet.mockResolvedValue(makeDashboard());
  });
  afterEach(() => cleanup());

  it('fetches dashboard on mount', async () => {
    const { result } = renderHook(() => useProjects());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/projects/dashboard');
    expect(result.current.dashboard).toEqual(makeDashboard());
    expect(result.current.activeTab).toBe('dashboard');
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useProjects());
    expect(result.current.loading).toBe(true);
  });

  it('sets error on failure', async () => {
    mockGet.mockRejectedValue(new Error('Server error'));
    const { result } = renderHook(() => useProjects());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Server error');
  });

  it('fetches keywords when switching to keywords tab', async () => {
    const { result } = renderHook(() => useProjects());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const kw = [{ keyword: 'react', count: 5 }];
    mockGet.mockResolvedValue(kw);

    act(() => { result.current.setActiveTab('keywords'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/projects/keywords');
    expect(result.current.keywords).toEqual(kw);
  });

  it('handles null keywords response', async () => {
    const { result } = renderHook(() => useProjects());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockResolvedValue(null);
    act(() => { result.current.setActiveTab('keywords'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.keywords).toEqual([]);
  });

  it('fetches dashboard for projects tab', async () => {
    const { result } = renderHook(() => useProjects());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue(makeDashboard({ total_projects: 10 }));

    act(() => { result.current.setActiveTab('projects'); });
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/projects/dashboard'));
  });

  it('fetches dashboard for timeline tab', async () => {
    const { result } = renderHook(() => useProjects());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue(makeDashboard());

    act(() => { result.current.setActiveTab('timeline'); });
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/projects/dashboard'));
  });

  it('refresh re-fetches current tab', async () => {
    const { result } = renderHook(() => useProjects());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue(makeDashboard({ total_projects: 20 }));

    act(() => { result.current.refresh(); });
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/projects/dashboard'));
  });
});
