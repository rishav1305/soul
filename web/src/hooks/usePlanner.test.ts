// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { usePlanner } from './usePlanner';

let wsHandler: ((msg: any) => void) | null = null;
const mockUnsubscribe = vi.fn();
const mockAuthFetch = vi.fn();

vi.mock('./useWebSocketContext.ts', () => ({
  useWebSocketCtx: () => ({
    onMessage: (handler: (msg: any) => void) => {
      wsHandler = handler;
      return mockUnsubscribe;
    },
    send: vi.fn(),
    connected: true,
  }),
}));

vi.mock('../lib/api.ts', () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

function makeTask(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Fix bug',
    description: 'Fix the login bug',
    stage: 'backlog',
    priority: 1,
    product: 'chat',
    workflow: 'micro',
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    ...overrides,
  };
}

describe('usePlanner', () => {
  beforeEach(() => {
    wsHandler = null;
    mockUnsubscribe.mockClear();
    mockAuthFetch.mockReset();
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([makeTask()]),
    });
  });
  afterEach(() => cleanup());

  it('fetches tasks on mount', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockAuthFetch).toHaveBeenCalledWith('/api/tasks');
    expect(result.current.tasks.length).toBe(1);
    expect(result.current.tasks[0].id).toBe(1);
  });

  it('starts in loading state', () => {
    mockAuthFetch.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => usePlanner());
    expect(result.current.loading).toBe(true);
  });

  it('groups tasks by stage', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([
        makeTask({ id: 1, stage: 'backlog' }),
        makeTask({ id: 2, stage: 'active' }),
        makeTask({ id: 3, stage: 'active' }),
        makeTask({ id: 4, stage: 'done' }),
      ]),
    });

    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.tasksByStage.backlog.length).toBe(1);
    expect(result.current.tasksByStage.active.length).toBe(2);
    expect(result.current.tasksByStage.done.length).toBe(1);
    expect(result.current.tasksByStage.brainstorm.length).toBe(0);
    expect(result.current.tasksByStage.blocked.length).toBe(0);
    expect(result.current.tasksByStage.validation.length).toBe(0);
  });

  it('adds task on task.created WS message', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const newTask = makeTask({ id: 2, title: 'New task' });
    act(() => {
      wsHandler!({ type: 'task.created', data: newTask });
    });

    expect(result.current.tasks.length).toBe(2);
    expect(result.current.tasks[1].title).toBe('New task');
  });

  it('updates task on task.updated WS message', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const updated = makeTask({ id: 1, title: 'Updated bug', stage: 'active' });
    act(() => {
      wsHandler!({ type: 'task.updated', data: updated });
    });

    expect(result.current.tasks[0].title).toBe('Updated bug');
    expect(result.current.tasks[0].stage).toBe('active');
  });

  it('updates task on task.stage_changed WS message', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const moved = makeTask({ id: 1, stage: 'validation' });
    act(() => {
      wsHandler!({ type: 'task.stage_changed', data: moved });
    });

    expect(result.current.tasks[0].stage).toBe('validation');
  });

  it('removes task on task.deleted WS message', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      wsHandler!({ type: 'task.deleted', data: { id: 1 } });
    });

    expect(result.current.tasks.length).toBe(0);
  });

  it('tracks task activities on task.activity WS message', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      wsHandler!({
        type: 'task.activity',
        data: {
          taskId: 1,
          activity: { taskId: 1, eventType: 'agent.end_turn', data: 'done' },
        },
      });
    });

    expect(result.current.taskActivities[1]?.length).toBe(1);
  });

  it('accumulates streaming output for tool_call events', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      wsHandler!({
        type: 'task.activity',
        data: {
          taskId: 1,
          activity: { taskId: 1, eventType: 'agent.tool_call', data: 'hello ' },
        },
      });
    });
    act(() => {
      wsHandler!({
        type: 'task.activity',
        data: {
          taskId: 1,
          activity: { taskId: 1, eventType: 'agent.tool_call', data: 'world' },
        },
      });
    });

    expect(result.current.taskStreams[1]).toBe('hello world');
  });

  it('clears stream on end_turn', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      wsHandler!({
        type: 'task.activity',
        data: {
          taskId: 1,
          activity: { taskId: 1, eventType: 'agent.tool_call', data: 'streaming...' },
        },
      });
    });
    expect(result.current.taskStreams[1]).toBe('streaming...');

    act(() => {
      wsHandler!({
        type: 'task.activity',
        data: {
          taskId: 1,
          activity: { taskId: 1, eventType: 'agent.end_turn', data: 'finished' },
        },
      });
    });

    expect(result.current.taskStreams[1]).toBeUndefined();
  });

  it('tracks task comments on task.comment WS message', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      wsHandler!({
        type: 'task.comment',
        data: {
          taskId: 1,
          comment: { taskId: 1, id: 'c1', author: 'user', body: 'looks good', type: 'feedback' },
        },
      });
    });

    expect(result.current.taskComments[1]?.length).toBe(1);
    expect(result.current.taskComments[1][0].body).toBe('looks good');
  });

  it('createTask posts and returns task', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const newTask = makeTask({ id: 2, title: 'New' });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(newTask),
    });

    let created: unknown;
    await act(async () => {
      created = await result.current.createTask('New', 'desc', 1, 'chat');
    });

    expect(created).toEqual(newTask);
    expect(mockAuthFetch).toHaveBeenCalledWith('/api/tasks', expect.objectContaining({
      method: 'POST',
    }));
  });

  it('updateTask patches task', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const updated = makeTask({ id: 1, title: 'Updated' });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(updated),
    });

    await act(async () => {
      await result.current.updateTask(1, { title: 'Updated' });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith('/api/tasks/1', expect.objectContaining({
      method: 'PATCH',
    }));
  });

  it('deleteTask sends DELETE request', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockAuthFetch.mockResolvedValueOnce({ ok: true });

    await act(async () => {
      await result.current.deleteTask(1);
    });

    expect(mockAuthFetch).toHaveBeenCalledWith('/api/tasks/1', expect.objectContaining({
      method: 'DELETE',
    }));
  });

  it('moveTask patches stage and posts comment', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const moved = makeTask({ id: 1, stage: 'active' });
    mockAuthFetch
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(moved) }) // PATCH stage
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({}) }); // POST comment

    await act(async () => {
      await result.current.moveTask(1, 'active', 'Starting work');
    });

    expect(mockAuthFetch).toHaveBeenCalledWith('/api/tasks/1', expect.objectContaining({
      method: 'PATCH',
      body: JSON.stringify({ stage: 'active' }),
    }));
    expect(mockAuthFetch).toHaveBeenCalledWith('/api/tasks/1/comments', expect.objectContaining({
      method: 'POST',
    }));
  });

  it('moveTask skips comment when empty', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const moved = makeTask({ id: 1, stage: 'done' });
    mockAuthFetch.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(moved) });

    await act(async () => {
      await result.current.moveTask(1, 'done', '');
    });

    // Only the PATCH, no comment POST
    const calls = mockAuthFetch.mock.calls.filter(c => String(c[0]).includes('/comments'));
    expect(calls.length).toBe(0);
  });

  it('fetchComments loads and stores comments', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const comments = [{ taskId: 1, id: 'c1', author: 'user', body: 'comment', type: 'feedback' }];
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(comments),
    });

    await act(async () => {
      await result.current.fetchComments(1);
    });

    expect(result.current.taskComments[1]).toEqual(comments);
  });

  it('addComment posts and returns comment', async () => {
    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const comment = { taskId: 1, id: 'c2', author: 'user', body: 'new comment', type: 'feedback' };
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(comment),
    });

    let added: unknown;
    await act(async () => {
      added = await result.current.addComment(1, 'new comment');
    });

    expect(added).toEqual(comment);
  });

  it('handles fetch failure gracefully', async () => {
    mockAuthFetch.mockRejectedValue(new Error('Network'));

    const { result } = renderHook(() => usePlanner());
    await waitFor(() => expect(result.current.loading).toBe(false));

    // Should not throw — tasks remain empty
    expect(result.current.tasks).toEqual([]);
  });
});
