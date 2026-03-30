// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useSessions } from './useSessions';

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

function makeSession(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Chat Session',
    summary: '',
    model: 'claude-3',
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    ...overrides,
  };
}

describe('useSessions', () => {
  beforeEach(() => {
    wsHandler = null;
    mockUnsubscribe.mockClear();
    mockAuthFetch.mockReset();
    localStorage.clear();
    // Default: fetchSessions returns sessions list
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sessions: [makeSession()] }),
    });
  });
  afterEach(() => cleanup());

  it('fetches sessions on mount', async () => {
    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));

    expect(mockAuthFetch).toHaveBeenCalledWith('/api/sessions');
    expect(result.current.sessions[0].id).toBe(1);
  });

  it('handles array response format', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([makeSession()]),
    });

    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
  });

  it('handles fetch failure gracefully', async () => {
    mockAuthFetch.mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useSessions());
    // Should not throw — just leave sessions empty
    await waitFor(() => expect(result.current.sessions).toEqual([]));
  });

  it('handles non-ok response', async () => {
    mockAuthFetch.mockResolvedValue({ ok: false });

    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions).toEqual([]));
  });

  it('createSession posts and adds to list', async () => {
    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));

    const newSession = makeSession({ id: 2, title: 'New Session' });
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ session: newSession }),
    });

    let created: unknown;
    await act(async () => {
      created = await result.current.createSession();
    });

    expect(created).toEqual(newSession);
    expect(result.current.sessions[0].id).toBe(2); // newest first
    expect(result.current.activeSessionId).toBe(2);
  });

  it('createSession throws on failure', async () => {
    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));

    mockAuthFetch.mockResolvedValueOnce({ ok: false });

    await expect(
      act(async () => {
        await result.current.createSession();
      }),
    ).rejects.toThrow('Failed to create session');
  });

  it('switchSession updates activeSessionId', async () => {
    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));

    act(() => {
      result.current.switchSession(5);
    });

    expect(result.current.activeSessionId).toBe(5);
  });

  it('persists activeSessionId to localStorage', async () => {
    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));

    act(() => {
      result.current.switchSession(42);
    });

    expect(localStorage.getItem('soul-active-session')).toBe('42');
  });

  it('loads activeSessionId from localStorage', async () => {
    localStorage.setItem('soul-active-session', '7');

    const { result } = renderHook(() => useSessions());
    expect(result.current.activeSessionId).toBe(7);
  });

  it('updates session on session.updated WS message', async () => {
    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));

    act(() => {
      wsHandler!({
        type: 'session.updated',
        data: { session_id: 1, title: 'Updated Title', summary: 'New summary', model: 'claude-4' },
      });
    });

    expect(result.current.sessions[0].title).toBe('Updated Title');
    expect(result.current.sessions[0].summary).toBe('New summary');
    expect(result.current.sessions[0].model).toBe('claude-4');
  });

  it('ignores non-session.updated WS messages', async () => {
    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));

    act(() => {
      wsHandler!({ type: 'task.created', data: {} });
    });

    // Title should be unchanged
    expect(result.current.sessions[0].title).toBe('Chat Session');
  });

  it('ignores session.updated for unknown session ids', async () => {
    const { result } = renderHook(() => useSessions());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));

    act(() => {
      wsHandler!({
        type: 'session.updated',
        data: { session_id: 999, title: 'Ghost' },
      });
    });

    // Should not crash or add new sessions
    expect(result.current.sessions.length).toBe(1);
    expect(result.current.sessions[0].title).toBe('Chat Session');
  });
});
