// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useTutor } from './useTutor';

const mockGet = vi.fn();

vi.mock('../lib/api', () => ({
  api: { get: (...args: unknown[]) => mockGet(...args) },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makeDashboard(overrides: Record<string, unknown> = {}) {
  return { total_topics: 10, mastered: 3, accuracy: 0.75, ...overrides };
}

function makeTopic(overrides: Record<string, unknown> = {}) {
  return { id: 1, name: 'Binary Trees', module: 'DSA', mastery: 0.8, ...overrides };
}

describe('useTutor', () => {
  beforeEach(() => {
    mockGet.mockReset();
    mockGet.mockResolvedValue(makeDashboard());
  });
  afterEach(() => cleanup());

  it('fetches dashboard on mount', async () => {
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/tutor/dashboard');
    expect(result.current.dashboard).toEqual(makeDashboard());
    expect(result.current.activeTab).toBe('dashboard');
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useTutor());
    expect(result.current.loading).toBe(true);
  });

  it('sets error on failure', async () => {
    mockGet.mockRejectedValue(new Error('Tutor down'));
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Tutor down');
  });

  it('fetches analytics on analytics tab', async () => {
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const analytics = { total_sessions: 50, avg_score: 0.8 };
    mockGet.mockResolvedValue(analytics);

    act(() => { result.current.setActiveTab('analytics'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/tutor/analytics');
    expect(result.current.analytics).toEqual(analytics);
  });

  it('fetches topics on topics tab', async () => {
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const topics = [makeTopic()];
    mockGet.mockResolvedValue({ topics });

    act(() => { result.current.setActiveTab('topics'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/tutor/topics');
    expect(result.current.topics).toEqual(topics);
  });

  it('topics fetches with module filter', async () => {
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockResolvedValue({ topics: [makeTopic({ module: 'AI' })] });

    act(() => {
      result.current.setModuleFilter('AI');
      result.current.setActiveTab('topics');
    });

    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/tutor/topics?module=AI'));
  });

  it('handles null topics response', async () => {
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockResolvedValue({ topics: null });
    act(() => { result.current.setActiveTab('topics'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.topics).toEqual([]);
  });

  it('fetches mocks on mocks tab', async () => {
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const sessions = [{ id: 's1', topic: 'DSA', status: 'completed' }];
    mockGet.mockResolvedValue({ sessions });

    act(() => { result.current.setActiveTab('mocks'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/tutor/mocks');
    expect(result.current.mocks).toEqual(sessions);
  });

  it('handles null mocks sessions', async () => {
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockResolvedValue({ sessions: null });
    act(() => { result.current.setActiveTab('mocks'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.mocks).toEqual([]);
  });

  it('refresh re-fetches current tab', async () => {
    const { result } = renderHook(() => useTutor());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue(makeDashboard({ total_topics: 20 }));

    act(() => { result.current.refresh(); });
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/tutor/dashboard'));
  });

  it('moduleFilter defaults to empty string', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useTutor());
    expect(result.current.moduleFilter).toBe('');
  });
});
