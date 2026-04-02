// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act } from '@testing-library/react';
import { useNotifications } from './useNotifications';

// Capture the WS message handler registered by the hook
let wsHandler: ((msg: any) => void) | null = null;
const mockUnsubscribe = vi.fn();

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
  uuid: () => 'test-uuid-' + Math.random().toString(36).slice(2, 8),
}));

function makeTasks() {
  return [
    { id: 1, title: 'Fix login bug' },
    { id: 2, title: 'Deploy API' },
  ];
}

function makeStageChangeMsg(taskId: number, fromStage: string, toStage: string) {
  return {
    type: 'task.activity',
    data: {
      taskId,
      activity: {
        taskId,
        eventType: 'task.stage_changed',
        data: `${fromStage} → ${toStage}`,
        createdAt: '2026-03-30T16:00:00Z',
      },
    },
  };
}

describe('useNotifications', () => {
  beforeEach(() => {
    wsHandler = null;
    mockUnsubscribe.mockClear();
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  it('returns empty toasts initially', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));
    expect(result.current.toasts).toEqual([]);
  });

  it('registers WS handler when enabled', () => {
    renderHook(() => useNotifications(makeTasks(), true));
    expect(wsHandler).not.toBeNull();
  });

  it('does not register WS handler when disabled', () => {
    renderHook(() => useNotifications(makeTasks(), false));
    expect(wsHandler).toBeNull();
  });

  it('creates toast on stage change message', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      wsHandler!(makeStageChangeMsg(1, 'backlog', 'active'));
    });

    expect(result.current.toasts.length).toBe(1);
    expect(result.current.toasts[0].taskId).toBe(1);
    expect(result.current.toasts[0].taskTitle).toBe('Fix login bug');
    expect(result.current.toasts[0].fromStage).toBe('backlog');
    expect(result.current.toasts[0].toStage).toBe('active');
  });

  it('uses task title fallback when task not found', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      wsHandler!(makeStageChangeMsg(99, 'backlog', 'active'));
    });

    expect(result.current.toasts[0].taskTitle).toBe('Task #99');
  });

  it('limits toasts to 5', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      for (let i = 0; i < 7; i++) {
        wsHandler!(makeStageChangeMsg(1, 'backlog', 'active'));
      }
    });

    expect(result.current.toasts.length).toBe(5);
  });

  it('newest toasts are first', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      wsHandler!(makeStageChangeMsg(1, 'backlog', 'active'));
    });
    act(() => {
      wsHandler!(makeStageChangeMsg(2, 'active', 'validation'));
    });

    expect(result.current.toasts[0].taskId).toBe(2);
    expect(result.current.toasts[1].taskId).toBe(1);
  });

  it('auto-dismisses after 4s', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      wsHandler!(makeStageChangeMsg(1, 'backlog', 'active'));
    });
    expect(result.current.toasts.length).toBe(1);

    act(() => {
      vi.advanceTimersByTime(4000);
    });

    expect(result.current.toasts.length).toBe(0);
  });

  it('dismiss removes specific toast', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      wsHandler!(makeStageChangeMsg(1, 'backlog', 'active'));
      wsHandler!(makeStageChangeMsg(2, 'active', 'done'));
    });

    const firstId = result.current.toasts[0].id;
    act(() => {
      result.current.dismiss(firstId);
    });

    expect(result.current.toasts.length).toBe(1);
    expect(result.current.toasts[0].id).not.toBe(firstId);
  });

  it('ignores non-task.activity messages', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      wsHandler!({ type: 'session.created', data: {} });
    });

    expect(result.current.toasts.length).toBe(0);
  });

  it('ignores non-stage_changed activities', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      wsHandler!({
        type: 'task.activity',
        data: {
          taskId: 1,
          activity: { taskId: 1, eventType: 'agent.tool_call', data: 'running tool' },
        },
      });
    });

    expect(result.current.toasts.length).toBe(0);
  });

  it('ignores malformed stage change data', () => {
    const { result } = renderHook(() => useNotifications(makeTasks(), true));

    act(() => {
      wsHandler!({
        type: 'task.activity',
        data: {
          taskId: 1,
          activity: { taskId: 1, eventType: 'task.stage_changed', data: 'no arrow here' },
        },
      });
    });

    expect(result.current.toasts.length).toBe(0);
  });

  it('unsubscribes on unmount', () => {
    const { unmount } = renderHook(() => useNotifications(makeTasks(), true));
    unmount();
    expect(mockUnsubscribe).toHaveBeenCalled();
  });
});
