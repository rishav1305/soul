// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, act, cleanup, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';

// ── Capture onMessage handler ──
let capturedOnMessage: ((msg: any) => void) | null = null;
const mockSend = vi.fn();
const mockOnMessage = vi.fn().mockImplementation((handler: (msg: any) => void) => {
  capturedOnMessage = handler;
  return () => { capturedOnMessage = null; };
});

vi.mock('./useWebSocketContext.ts', () => ({
  useWebSocketCtx: () => ({
    send: mockSend,
    onMessage: mockOnMessage,
    connected: true,
  }),
}));

// ── Mock authFetch ──
const mockAuthFetch = vi.fn();
const mockUuid = vi.fn().mockReturnValue('test-uuid');

vi.mock('../lib/api.ts', () => ({
  authFetch: (...args: any[]) => mockAuthFetch(...args),
  uuid: () => mockUuid(),
}));

import { ChatSessionsProvider, useChatSessions } from './useChatSessions';

// ── Test wrapper ──
function wrapper({ children }: { children: ReactNode }) {
  return <ChatSessionsProvider>{children}</ChatSessionsProvider>;
}

// Helper to fire WS message through captured handler
function fireWS(type: string, data?: Record<string, unknown>, sessionId?: number | string) {
  if (!capturedOnMessage) throw new Error('onMessage not captured');
  act(() => {
    capturedOnMessage!({ type, data, sessionId, content: data?.content ?? undefined });
  });
}

describe('useChatSessions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedOnMessage = null;
    // Default: fetchSessions returns empty list
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sessions: [] }),
    });
    // Clear localStorage
    localStorage.clear();
  });
  afterEach(() => cleanup());

  // ─── Provider / Context ───
  it('throws when used outside provider', () => {
    // Suppress console.error for expected error
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => {
      renderHook(() => useChatSessions());
    }).toThrow('useChatSessions must be used inside <ChatSessionsProvider>');
    spy.mockRestore();
  });

  it('provides initial state inside provider', () => {
    const { result } = renderHook(() => useChatSessions(), { wrapper });
    expect(result.current.sessions).toEqual([]);
    expect(result.current.activeSessionId).toBeNull();
    expect(result.current.messages).toEqual([]);
    expect(result.current.isStreaming).toBe(false);
    expect(result.current.connected).toBe(true);
  });

  // ─── Session list ───
  it('fetches sessions on mount', async () => {
    const sessions = [{ id: 1, title: 'Chat 1', status: 'idle' }];
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sessions }),
    });

    const { result } = renderHook(() => useChatSessions(), { wrapper });

    await waitFor(() => {
      expect(result.current.sessions).toEqual(sessions);
    });
    expect(mockAuthFetch).toHaveBeenCalledWith('/api/sessions');
  });

  // ─── Active session management ───
  it('switches active session and saves to localStorage', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sessions: [], messages: [] }),
    });

    const { result } = renderHook(() => useChatSessions(), { wrapper });

    act(() => {
      result.current.setActiveSessionId(42);
    });

    expect(result.current.activeSessionId).toBe(42);
    expect(localStorage.getItem('soul-active-session')).toBe('42');
  });

  it('marks session read when switching', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sessions: [], messages: [] }),
    });

    const { result } = renderHook(() => useChatSessions(), { wrapper });

    act(() => {
      result.current.setActiveSessionId(7);
    });

    expect(mockAuthFetch).toHaveBeenCalledWith('/api/sessions/7/read', { method: 'PATCH' });
  });

  it('creates new session (nulls active ID)', () => {
    const { result } = renderHook(() => useChatSessions(), { wrapper });

    // Set active first
    act(() => {
      result.current.setActiveSessionId(10);
    });
    expect(result.current.activeSessionId).toBe(10);

    // Create new
    act(() => {
      result.current.createSession();
    });
    expect(result.current.activeSessionId).toBeNull();
    expect(localStorage.getItem('soul-active-session')).toBeNull();
  });

  // ─── Send message (existing session) ───
  it('sends message on existing session via WS', () => {
    const { result } = renderHook(() => useChatSessions(), { wrapper });

    act(() => {
      result.current.setActiveSessionId(5);
    });

    act(() => {
      result.current.sendMessage('Hello world');
    });

    // Should add optimistic user message and set isStreaming
    expect(result.current.isStreaming).toBe(true);
    expect(result.current.messages.length).toBe(1);
    expect(result.current.messages[0]!.role).toBe('user');
    expect(result.current.messages[0]!.content).toBe('Hello world');

    // Should send via WS
    expect(mockSend).toHaveBeenCalledWith(expect.objectContaining({
      type: 'chat.send',
      content: 'Hello world',
      sessionId: '5',
    }));
  });

  it('sends message with model option', () => {
    const { result } = renderHook(() => useChatSessions(), { wrapper });

    act(() => {
      result.current.setActiveSessionId(5);
    });

    act(() => {
      result.current.sendMessage('Test', { model: 'claude-3-opus' });
    });

    expect(mockSend).toHaveBeenCalledWith(expect.objectContaining({
      type: 'chat.send',
      model: 'claude-3-opus',
    }));
  });

  // ─── Send message (deferred session creation) ───
  it('creates session via REST when no active session', async () => {
    // Session creation via REST
    mockAuthFetch
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({ sessions: [] }) }) // initial fetch
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({ session: { id: 100 } }) }) // POST create
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({ sessions: [{ id: 100, title: '' }] }) }); // refresh

    const { result } = renderHook(() => useChatSessions(), { wrapper });

    act(() => {
      result.current.sendMessage('First message');
    });

    // Wait for the async session creation
    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledWith('/api/sessions', expect.objectContaining({
        method: 'POST',
      }));
    });

    await waitFor(() => {
      expect(mockSend).toHaveBeenCalledWith(expect.objectContaining({
        type: 'chat.send',
        content: 'First message',
        sessionId: '100',
      }));
    });
  });

  // ─── Stop streaming ───
  it('stops streaming and sends chat.stop', () => {
    const { result } = renderHook(() => useChatSessions(), { wrapper });

    act(() => {
      result.current.setActiveSessionId(5);
    });

    // Start streaming
    act(() => {
      result.current.sendMessage('Test');
    });
    expect(result.current.isStreaming).toBe(true);

    // Stop
    act(() => {
      result.current.stopStreaming();
    });

    expect(mockSend).toHaveBeenCalledWith(expect.objectContaining({
      type: 'chat.stop',
      sessionId: '5',
    }));
    expect(result.current.isStreaming).toBe(false);
  });

  // ─── WS message handling ───
  describe('WS message routing', () => {
    it('handles chat.token — appends to existing assistant message', () => {
      const { result } = renderHook(() => useChatSessions(), { wrapper });

      act(() => {
        result.current.setActiveSessionId(1);
      });

      // Send a message first (creates user msg + sets streaming)
      act(() => {
        result.current.sendMessage('Test');
      });

      // First token — creates assistant message
      fireWS('chat.token', { token: 'Hello' }, 1);
      expect(result.current.messages.length).toBe(2);
      expect(result.current.messages[1]!.role).toBe('assistant');
      expect(result.current.messages[1]!.content).toBe('Hello');

      // Second token — appends to existing
      fireWS('chat.token', { token: ' world' }, 1);
      expect(result.current.messages[1]!.content).toBe('Hello world');
    });

    it('handles chat.thinking — sets thinking field', () => {
      const { result } = renderHook(() => useChatSessions(), { wrapper });

      act(() => {
        result.current.setActiveSessionId(1);
        result.current.sendMessage('Test');
      });

      fireWS('chat.thinking', { text: 'Let me think...' }, 1);
      const last = result.current.messages[result.current.messages.length - 1]!;
      expect(last.thinking).toBe('Let me think...');
    });

    it('handles chat.done — stops streaming, sets tokenUsage', () => {
      const { result } = renderHook(() => useChatSessions(), { wrapper });

      act(() => {
        result.current.setActiveSessionId(1);
        result.current.sendMessage('Test');
      });

      expect(result.current.isStreaming).toBe(true);

      fireWS('chat.done', { input_tokens: 500, output_tokens: 200, context_pct: 10 }, 1);
      expect(result.current.isStreaming).toBe(false);
      expect(result.current.tokenUsage).toEqual({
        inputTokens: 500,
        outputTokens: 200,
        contextPct: 10,
      });
    });

    it('handles tool.call — adds tool call to assistant message', () => {
      const { result } = renderHook(() => useChatSessions(), { wrapper });

      act(() => {
        result.current.setActiveSessionId(1);
        result.current.sendMessage('Test');
      });

      // Add an assistant message first
      fireWS('chat.token', { token: 'I will use a tool' }, 1);

      fireWS('tool.call', { id: 'tc-1', name: 'search', input: { q: 'test' } }, 1);

      const last = result.current.messages[result.current.messages.length - 1]!;
      expect(last.toolCalls).toBeDefined();
      expect(last.toolCalls!.length).toBe(1);
      expect(last.toolCalls![0]!.name).toBe('search');
      expect(last.toolCalls![0]!.status).toBe('running');
    });

    it('handles tool.complete — updates tool call status', () => {
      const { result } = renderHook(() => useChatSessions(), { wrapper });

      act(() => {
        result.current.setActiveSessionId(1);
        result.current.sendMessage('Test');
      });

      fireWS('chat.token', { token: 'Using tool' }, 1);
      fireWS('tool.call', { id: 'tc-1', name: 'search', input: {} }, 1);
      fireWS('tool.complete', { id: 'tc-1', result: 'found 5 results' }, 1);

      const last = result.current.messages[result.current.messages.length - 1]!;
      expect(last.toolCalls![0]!.status).toBe('complete');
      expect(last.toolCalls![0]!.result).toBe('found 5 results');
    });

    it('handles tool.error — marks tool call as error', () => {
      const { result } = renderHook(() => useChatSessions(), { wrapper });

      act(() => {
        result.current.setActiveSessionId(1);
        result.current.sendMessage('Test');
      });

      fireWS('chat.token', { token: 'Using tool' }, 1);
      fireWS('tool.call', { id: 'tc-2', name: 'run', input: {} }, 1);
      fireWS('tool.error', { id: 'tc-2', error: 'timeout' }, 1);

      const last = result.current.messages[result.current.messages.length - 1]!;
      expect(last.toolCalls![0]!.status).toBe('error');
      expect(last.toolCalls![0]!.error).toBe('timeout');
    });

    it('handles tool.progress — updates progress fields', () => {
      const { result } = renderHook(() => useChatSessions(), { wrapper });

      act(() => {
        result.current.setActiveSessionId(1);
        result.current.sendMessage('Test');
      });

      fireWS('chat.token', { token: 'Working...' }, 1);
      fireWS('tool.call', { id: 'tc-3', name: 'build', input: {} }, 1);
      fireWS('tool.progress', { id: 'tc-3', progress: 50, message: 'Halfway' }, 1);

      const last = result.current.messages[result.current.messages.length - 1]!;
      expect(last.toolCalls![0]!.progress).toBe(50);
      expect(last.toolCalls![0]!.progressMessage).toBe('Halfway');
    });

    it('handles session.updated — updates session metadata', async () => {
      mockAuthFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({
          sessions: [{ id: 1, title: 'Old Title', status: 'idle' }],
        }),
      });

      const { result } = renderHook(() => useChatSessions(), { wrapper });

      await waitFor(() => {
        expect(result.current.sessions.length).toBe(1);
      });

      fireWS('session.updated', { session: { id: 1, title: 'New Title', summary: 'Summary' } });

      expect(result.current.sessions[0]!.title).toBe('New Title');
      expect((result.current.sessions[0] as any).summary).toBe('Summary');
    });
  });

  // ─── Computed values ───
  describe('computed values', () => {
    it('computes runningSessions from session status', async () => {
      mockAuthFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({
          sessions: [
            { id: 1, title: 'Running', status: 'running' },
            { id: 2, title: 'Idle', status: 'idle' },
          ],
        }),
      });

      const { result } = renderHook(() => useChatSessions(), { wrapper });

      await waitFor(() => {
        expect(result.current.runningSessions.length).toBe(1);
      });
      expect(result.current.runningSessions[0]!.title).toBe('Running');
    });

    it('computes unreadSessions from session status', async () => {
      mockAuthFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({
          sessions: [
            { id: 1, title: 'Unread', status: 'completed_unread' },
            { id: 2, title: 'Read', status: 'idle' },
          ],
        }),
      });

      const { result } = renderHook(() => useChatSessions(), { wrapper });

      await waitFor(() => {
        expect(result.current.unreadSessions.length).toBe(1);
      });
      expect(result.current.unreadSessions[0]!.title).toBe('Unread');
    });

    it('returns empty session state for nonexistent session', () => {
      const { result } = renderHook(() => useChatSessions(), { wrapper });
      const state = result.current.getSessionState(999);
      expect(state.messages).toEqual([]);
      expect(state.isStreaming).toBe(false);
      expect(state.tokenUsage).toBeNull();
    });
  });

  // ─── localStorage restore ───
  it('restores active session from localStorage on mount', async () => {
    localStorage.setItem('soul-active-session', '42');

    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sessions: [], messages: [] }),
    });

    const { result } = renderHook(() => useChatSessions(), { wrapper });

    // Should restore 42 from localStorage
    expect(result.current.activeSessionId).toBe(42);
  });
});
