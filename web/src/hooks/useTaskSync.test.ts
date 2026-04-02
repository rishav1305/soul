// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useTaskSync } from './useTaskSync';

const mockGet = vi.fn();
const mockPost = vi.fn();
const mockPatch = vi.fn();
const mockDelete = vi.fn();

vi.mock('../lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockGet(...args),
    post: (...args: unknown[]) => mockPost(...args),
    patch: (...args: unknown[]) => mockPatch(...args),
    delete: (...args: unknown[]) => mockDelete(...args),
  },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makeTask(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Fix bug',
    description: 'Fix it',
    stage: 'backlog',
    priority: 1,
    product: 'chat',
    seq: 1,
    ...overrides,
  };
}

function makeSyncResponse(tasks: unknown[] = [makeTask()], overrides: Record<string, unknown> = {}) {
  return {
    tasks,
    deleted: [],
    cursor: 'cursor-1',
    fullSync: true,
    ...overrides,
  };
}

function dispatchTaskEvent(type: string, data: unknown) {
  window.dispatchEvent(new CustomEvent('ws:task-event', {
    detail: { type, data },
  }));
}

describe('useTaskSync', () => {
  beforeEach(() => {
    mockGet.mockReset();
    mockPost.mockReset();
    mockPatch.mockReset();
    mockDelete.mockReset();
    // Default: full sync on mount
    mockGet.mockResolvedValue(makeSyncResponse());
  });
  afterEach(() => {
    cleanup();
  });

  it('does full sync on mount', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/tasks/sync');
    expect(result.current.tasks.length).toBe(1);
    expect(result.current.tasks[0].id).toBe(1);
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useTaskSync());
    expect(result.current.loading).toBe(true);
  });

  it('sets error on sync failure', async () => {
    mockGet.mockRejectedValue(new Error('Sync failed'));
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Sync failed');
  });

  it('sets connected true after successful sync', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.connected).toBe(true);
  });

  it('applies task update from WS event', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      dispatchTaskEvent('task.updated', makeTask({ id: 1, title: 'Updated', seq: 2 }));
    });

    expect(result.current.tasks[0].title).toBe('Updated');
  });

  it('ignores stale task update (lower seq)', async () => {
    mockGet.mockResolvedValue(makeSyncResponse([makeTask({ id: 1, seq: 5 })]));
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      dispatchTaskEvent('task.updated', makeTask({ id: 1, title: 'Stale', seq: 3 }));
    });

    expect(result.current.tasks[0].title).toBe('Fix bug');
  });

  it('adds task from task.created WS event', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      dispatchTaskEvent('task.created', makeTask({ id: 2, title: 'New', seq: 1 }));
    });

    expect(result.current.tasks.length).toBe(2);
  });

  it('removes task from task.deleted WS event', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      dispatchTaskEvent('task.deleted', { id: 1 });
    });

    expect(result.current.tasks.length).toBe(0);
  });

  it('createTask posts and applies update', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const newTask = makeTask({ id: 2, title: 'Created', seq: 1 });
    mockPost.mockResolvedValue(newTask);

    let created: unknown;
    await act(async () => {
      created = await result.current.createTask({ title: 'Created' });
    });

    expect(mockPost).toHaveBeenCalledWith('/api/tasks', { title: 'Created' });
    expect(created).toEqual(newTask);
    expect(result.current.tasks.find(t => t.id === 2)).toBeTruthy();
  });

  it('updateTask patches and applies update', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const updated = makeTask({ id: 1, title: 'Patched', seq: 2 });
    mockPatch.mockResolvedValue(updated);

    await act(async () => {
      await result.current.updateTask(1, { title: 'Patched' });
    });

    expect(mockPatch).toHaveBeenCalledWith('/api/tasks/1', { title: 'Patched' });
    expect(result.current.tasks[0].title).toBe('Patched');
  });

  it('deleteTask optimistically removes and calls API', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.tasks.length).toBe(1);

    mockDelete.mockResolvedValue(undefined);

    await act(async () => {
      await result.current.deleteTask(1);
    });

    expect(mockDelete).toHaveBeenCalledWith('/api/tasks/1');
    expect(result.current.tasks.length).toBe(0);
  });

  it('startTask posts to start endpoint', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(undefined);

    await act(async () => {
      await result.current.startTask(1);
    });

    expect(mockPost).toHaveBeenCalledWith('/api/tasks/1/start');
  });

  it('stopTask posts to stop endpoint', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(undefined);

    await act(async () => {
      await result.current.stopTask(1);
    });

    expect(mockPost).toHaveBeenCalledWith('/api/tasks/1/stop');
  });

  it('addComment posts to comments endpoint', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(undefined);

    await act(async () => {
      await result.current.addComment(1, 'Great work');
    });

    expect(mockPost).toHaveBeenCalledWith('/api/tasks/1/comments', {
      author: 'user',
      type: 'feedback',
      body: 'Great work',
    });
  });

  it('refresh triggers full sync', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue(makeSyncResponse([makeTask({ id: 3 })]));

    await act(async () => {
      result.current.refresh();
    });

    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/tasks/sync'));
  });

  it('returns specific task in detail mode', async () => {
    mockGet.mockImplementation((path: string) => {
      if (path === '/api/tasks/sync') return Promise.resolve(makeSyncResponse([makeTask({ id: 1 }), makeTask({ id: 2, title: 'Other' })]));
      if (path.includes('/activity')) return Promise.resolve([]);
      if (path.includes('/comments')) return Promise.resolve([]);
      return Promise.resolve(null);
    });

    const { result } = renderHook(() => useTaskSync({ taskId: 1, mode: 'detail' }));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.task?.id).toBe(1);
    expect(result.current.tasks.length).toBe(2);
  });

  it('task is null when no taskId provided', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.task).toBeNull();
  });

  it('handles stage_changed WS event', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      dispatchTaskEvent('task.stage_changed', makeTask({ id: 1, stage: 'active', seq: 2 }));
    });

    expect(result.current.tasks[0].stage).toBe('active');
  });

  it('handles substep_changed WS event', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      dispatchTaskEvent('task.substep_changed', makeTask({ id: 1, title: 'Substep Updated', seq: 2 }));
    });

    expect(result.current.tasks[0].title).toBe('Substep Updated');
  });

  it('parses JSON string data in WS events', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      dispatchTaskEvent('task.updated', JSON.stringify(makeTask({ id: 1, title: 'Parsed', seq: 2 })));
    });

    expect(result.current.tasks[0].title).toBe('Parsed');
  });

  it('ws:connected event sets connected true', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      window.dispatchEvent(new Event('ws:disconnected'));
    });
    expect(result.current.connected).toBe(false);

    act(() => {
      window.dispatchEvent(new Event('ws:connected'));
    });
    expect(result.current.connected).toBe(true);
  });

  it('ws:disconnected sets connected false', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.connected).toBe(true);

    act(() => {
      window.dispatchEvent(new Event('ws:disconnected'));
    });
    expect(result.current.connected).toBe(false);
  });

  it('deleteTask re-syncs on API failure', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockDelete.mockRejectedValue(new Error('Delete failed'));
    // After failed delete, full sync should restore task
    mockGet.mockResolvedValue(makeSyncResponse([makeTask({ id: 1, title: 'Restored' })]));

    await expect(act(async () => {
      await result.current.deleteTask(1);
    })).rejects.toThrow('Delete failed');

    // Full sync re-fetched and restored
    await waitFor(() => expect(result.current.tasks.length).toBe(1));
  });

  it('handles non-Error objects in sync failure', async () => {
    mockGet.mockRejectedValue('network error string');
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Sync failed');
  });

  it('detail mode fetches activities and comments', async () => {
    const activity = { id: 1, taskId: 1, type: 'stage_change', data: '{}', createdAt: '2026-03-30T10:00:00Z' };
    const comment = { id: 1, taskId: 1, author: 'user', type: 'feedback', body: 'Hello', createdAt: '2026-03-30T10:00:00Z' };

    mockGet.mockImplementation((path: string) => {
      if (path === '/api/tasks/sync') return Promise.resolve(makeSyncResponse([makeTask()]));
      if (path.includes('/activity')) return Promise.resolve([activity]);
      if (path.includes('/comments')) return Promise.resolve([comment]);
      return Promise.resolve(null);
    });

    const { result } = renderHook(() => useTaskSync({ taskId: 1, mode: 'detail' }));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.activities.length).toBe(1);
    expect(result.current.comments.length).toBe(1);
    expect(result.current.comments[0].body).toBe('Hello');
  });

  it('activities are returned in reverse order', async () => {
    const act1 = { id: 1, taskId: 1, type: 'created', data: '{}', createdAt: '2026-03-30T10:00:00Z' };
    const act2 = { id: 2, taskId: 1, type: 'stage_change', data: '{}', createdAt: '2026-03-30T11:00:00Z' };

    mockGet.mockImplementation((path: string) => {
      if (path === '/api/tasks/sync') return Promise.resolve(makeSyncResponse([makeTask()]));
      if (path.includes('/activity')) return Promise.resolve([act1, act2]);
      if (path.includes('/comments')) return Promise.resolve([]);
      return Promise.resolve(null);
    });

    const { result } = renderHook(() => useTaskSync({ taskId: 1, mode: 'detail' }));
    await waitFor(() => expect(result.current.loading).toBe(false));

    // Activities reversed for UI — newest first
    expect(result.current.activities[0].id).toBe(2);
    expect(result.current.activities[1].id).toBe(1);
  });

  it('ignores WS events with no detail type', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    // Event with empty detail — should not crash
    act(() => {
      window.dispatchEvent(new CustomEvent('ws:task-event', { detail: {} }));
    });

    expect(result.current.tasks.length).toBe(1);
  });

  it('ignores duplicate task.deleted for non-existent task', async () => {
    const { result } = renderHook(() => useTaskSync());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      dispatchTaskEvent('task.deleted', { id: 999 });
    });

    // Still has original task
    expect(result.current.tasks.length).toBe(1);
  });
});
